package sale

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SaleService) CompleteSale(
	ctx context.Context,
	req *connect.Request[posifacev1.CompleteSaleRequest],
) (*connect.Response[posifacev1.CompleteSaleResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	paymentStr, err := paymentSourceToString(req.Msg.PaymentSource)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}

		var items []model.SaleItem
		if err := tx.Where("sale_id = ?", sale.ID).Order("created_at").Find(&items).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if len(items) == 0 {
			return common.TokenError(connect.CodeFailedPrecondition, "sale.cart_empty")
		}

		// Pharmacy: re-assert Rx coverage at completion (per distinct product) so
		// a DRAFT built while the gate was off — retail mode at add-time, then a
		// switch to pharmacy mode (license re-applied on boot; drafts survive) —
		// can't finalize an Rx-required product with no covering prescription.
		// No-op in retail / for non-Rx products (assertRxCovers is mode+flag gated).
		seenRx := make(map[string]struct{}, len(items))
		for i := range items {
			pid := items[i].ProductID
			if _, ok := seenRx[pid]; ok {
				continue
			}
			seenRx[pid] = struct{}{}
			if err := s.assertRxCovers(ctx, tx, sale, pid); err != nil {
				return err
			}
		}

		// Cash requires paid_amount >= total. Non-cash settles externally.
		if paymentStr == paymentCash && req.Msg.PaidAmount < sale.Total {
			return common.TokenError(connect.CodeInvalidArgument, "sale.paid_too_low")
		}

		now := time.Now()

		// FEFO consumes stock from the sale's warehouse only.
		saleWh := common.Deref(sale.WarehouseID)

		// Expand the cart into what it actually takes out of stock: a STOCKED line
		// consumes itself, a COMPOSITE (menu item / bundle) consumes its recipe's
		// components, a SERVICE line consumes nothing. Every resulting movement
		// still carries the SALE LINE's id, so COGS — which joins SALE movements
		// by sale_item_id — keeps working unchanged for exploded lines.
		cons, err := resolveConsumption(tx, items)
		if err != nil {
			return err
		}

		// Lock every lot of the CONSUMED products FOR UPDATE (deterministic id
		// order) BEFORE reading availability, so concurrent CompleteSale /
		// transfer / adjustment for the same lot serialize and can't oversell.
		// For a composite cart these are the ingredients, not the menu items.
		if err := common.LockBatchesByProduct(tx, consumedProductIDs(cons)); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		// Consume each instruction's BASE-unit quantity across FEFO batches in the
		// sale's warehouse.
		for _, c := range cons {
			needed := c.NeedBase

			var batches []model.Batch
			if err := tx.Where("product_id = ?", c.ProductID).
				Order("expiry_date ASC").
				Find(&batches).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			for _, b := range batches {
				if needed <= 0 {
					break
				}
				var avail int64
				if err := tx.Model(&model.StockMovement{}).
					Where("batch_id = ? AND warehouse_id = ?", b.ID, saleWh).
					Select("COALESCE(SUM(qty), 0)").
					Scan(&avail).Error; err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}
				if avail <= 0 {
					continue
				}
				take := int64(needed)
				if take > avail {
					take = avail
				}

				saleItemID := c.SaleItemID
				mv := model.StockMovement{
					BatchID:     b.ID,
					Qty:         -int32(take),
					Type:        movementTypeSale,
					Reason:      "POS sale",
					UserID:      caller.UserID,
					SaleItemID:  &saleItemID,
					WarehouseID: saleWh,
				}
				if err := tx.Create(&mv).Error; err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}

				needed -= int32(take)
			}

			if needed > 0 {
				return connect.NewError(connect.CodeFailedPrecondition,
					fmt.Errorf("insufficient stock for product %s (%d base units short)", c.ProductID, needed))
			}
		}

		// Pharmacy: accrue dispensed_qty against the attached Rx (no-op if none).
		if err := s.incrementRxDispensed(tx, sale, items); err != nil {
			return err
		}

		// Recompute totals from the now-allocated sale_items.
		if err := recomputeSaleTotals(tx, sale.ID); err != nil {
			return err
		}

		// Assign per-year sale_no.
		saleNo, err := assignSaleNo(tx, now)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		updates := map[string]any{
			"sale_no":        saleNo,
			"payment_source": paymentStr,
			"paid_amount":    req.Msg.PaidAmount,
			"status":         saleStatusCompleted,
			"completed_at":   now,
		}
		if err := tx.Model(sale).Updates(updates).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		sale.Status = saleStatusCompleted
		sale.CompletedAt = &now
		sale.SaleNo = &saleNo
		sale.PaymentSource = &paymentStr
		sale.PaidAmount = req.Msg.PaidAmount

		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.CompleteSaleResponse{Sale: saleToProto(sale)}), nil
}
