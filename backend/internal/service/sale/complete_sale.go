package sale

import (
	"context"
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

		// Lock every lot of the cart's products FOR UPDATE (deterministic id
		// order) BEFORE reading availability, so concurrent CompleteSale /
		// transfer / adjustment for the same lot serialize and can't oversell.
		medSet := make(map[string]struct{}, len(items))
		medIDs := make([]string, 0, len(items))
		for i := range items {
			if _, ok := medSet[items[i].ProductID]; ok {
				continue
			}
			medSet[items[i].ProductID] = struct{}{}
			medIDs = append(medIDs, items[i].ProductID)
		}
		if err := common.LockBatchesByProduct(tx, medIDs); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		// Pharmacy: a lot whose expiry date has passed is never sold. It is
		// still sellable ON its expiry date (common.ExpiryPassed compares shop-
		// local calendar days), and a "does not expire" lot carries
		// common.NoExpiryDate, so it never trips this. Retail sells FEFO as it
		// always has. The mode is read once for the whole cart.
		pharmacy, err := common.IsPharmacyMode(ctx, tx)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		// For each line: consume its BASE-unit quantity across FEFO batches in the
		// sale's warehouse.
		for i := range items {
			item := items[i]
			needed := item.BaseQty
			if needed <= 0 {
				needed = item.Qty // back-compat for any rows created before UOM
			}

			var batches []model.Batch
			if err := tx.Where("product_id = ?", item.ProductID).
				Order("expiry_date ASC").
				Find(&batches).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			// Stock of this line's product, in the sale's warehouse, that FEFO
			// skipped because its lot had expired (pharmacy only). Expired lots
			// sort first, so every one of them is seen before the line is covered.
			var expired int64
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
				if pharmacy && common.ExpiryPassed(b.ExpiryDate, now) {
					expired += avail
					continue
				}
				take := int64(needed)
				if take > avail {
					take = avail
				}

				saleItemID := item.ID
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

			// Short: refuse the whole sale (the tx rolls back every movement
			// written so far). There is no override for expired stock: the fix
			// is a write-off, or SetBatchExpiry when the date was typed wrong.
			if needed > 0 {
				if expired > 0 {
					return common.TokenError(connect.CodeFailedPrecondition, "sale.stock_expired")
				}
				return common.TokenError(connect.CodeFailedPrecondition, "sale.insufficient_stock")
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
