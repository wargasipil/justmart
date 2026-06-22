package sale

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// refundWindow bounds how long after completion a sale may be refunded. Past
// this, the refund is rejected (stale returns must be handled manually).
const refundWindow = 24 * time.Hour

// RefundSale fully refunds a COMPLETED order. It flips the sale to REFUNDED,
// records the refunded amount/reason, and — when restock is requested — returns
// the goods to stock (a positive RETURN movement mirroring each of the sale's
// SALE movements) and reverses pharmacy Rx dispensing. Money-only refunds
// (restock=false) leave stock + Rx dispensing untouched. Manager-tier
// (OWNER+PHARMACIST, proto-declared). Terminal: a REFUNDED sale can't be
// refunded again.
//
// Analytics/summary/today-snapshot all filter status='COMPLETED' and COGS keys
// off type='SALE', so a refunded order drops out of revenue/COGS automatically
// and the RETURN movement is never counted as COGS.
func (s *SaleService) RefundSale(
	ctx context.Context,
	req *connect.Request[posifacev1.RefundSaleRequest],
) (*connect.Response[posifacev1.RefundSaleResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sale model.Sale
		if err := common.RowLock(tx).Where("id = ?", req.Msg.SaleId).First(&sale).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("sale not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if sale.Status != saleStatusCompleted {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only completed sales can be refunded; this one is %s", sale.Status))
		}
		// Refunds are time-boxed: only within refundWindow (1 day) of completion.
		if sale.CompletedAt == nil || now.Sub(*sale.CompletedAt) > refundWindow {
			return connect.NewError(connect.CodeFailedPrecondition,
				errors.New("refunds are only allowed within 1 day of completion"))
		}

		var items []model.SaleItem
		if err := tx.Where("sale_id = ?", sale.ID).Find(&items).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		if req.Msg.Restock {
			if err := s.restockSaleItems(tx, &sale, items, caller.UserID); err != nil {
				return err
			}
			// Reverse Rx dispensing only when the goods came back (money-only
			// refunds mean the patient kept the meds → dispense stands).
			if err := s.decrementRxDispensed(tx, &sale, items); err != nil {
				return err
			}
		}

		return tx.Model(&sale).Updates(map[string]any{
			"status":           saleStatusRefunded,
			"refunded_at":      now,
			"refund_amount":    sale.Total,
			"refund_reason":    req.Msg.Reason,
			"refund_restocked": req.Msg.Restock,
		}).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.RefundSaleResponse{Sale: saleToProto(sale)}), nil
}

// restockSaleItems writes a positive RETURN movement mirroring each SALE
// movement the sale consumed (same batch + warehouse + linked sale_item), so the
// exact lots the FEFO allocation took are put back. Additive, so no availability
// check; locks the affected batches for consistency with the rest of the ledger.
func (s *SaleService) restockSaleItems(tx *gorm.DB, sale *model.Sale, items []model.SaleItem, userID string) error {
	if len(items) == 0 {
		return nil
	}
	itemIDs := make([]string, 0, len(items))
	for i := range items {
		itemIDs = append(itemIDs, items[i].ID)
	}

	var saleMovements []model.StockMovement
	if err := tx.Where("sale_item_id IN ? AND type = ?", itemIDs, movementTypeSale).
		Find(&saleMovements).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	batchIDs := make([]string, 0, len(saleMovements))
	seen := make(map[string]struct{}, len(saleMovements))
	for i := range saleMovements {
		if _, ok := seen[saleMovements[i].BatchID]; ok {
			continue
		}
		seen[saleMovements[i].BatchID] = struct{}{}
		batchIDs = append(batchIDs, saleMovements[i].BatchID)
	}
	if err := common.LockBatchesByID(tx, batchIDs); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	reason := "Refund"
	if sale.SaleNo != nil && *sale.SaleNo != "" {
		reason = "Refund " + *sale.SaleNo
	}
	for i := range saleMovements {
		mv := saleMovements[i]
		ret := model.StockMovement{
			BatchID:     mv.BatchID,
			Qty:         -mv.Qty, // SALE qty is negative → RETURN is positive
			Type:        movementTypeReturn,
			Reason:      reason,
			UserID:      userID,
			SaleItemID:  mv.SaleItemID,
			WarehouseID: mv.WarehouseID,
		}
		if err := tx.Create(&ret).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}
	return nil
}

// decrementRxDispensed reverses incrementRxDispensed for a restock refund: it
// subtracts the sale's per-product base qty back off prescription_items.
// dispensed_qty (floored at 0 to guard underflow). No-op when no Rx is attached.
func (s *SaleService) decrementRxDispensed(tx *gorm.DB, sale *model.Sale, items []model.SaleItem) error {
	if sale.PrescriptionID == nil || *sale.PrescriptionID == "" {
		return nil
	}

	perProduct := map[string]int32{}
	for i := range items {
		base := items[i].BaseQty
		if base <= 0 {
			base = items[i].Qty // back-compat for pre-UOM rows
		}
		perProduct[items[i].ProductID] += base
	}

	var rx model.Prescription
	if err := common.RowLock(tx).Preload("Items").
		Where("id = ?", *sale.PrescriptionID).First(&rx).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	for i := range rx.Items {
		it := &rx.Items[i]
		back, ok := perProduct[it.ProductID]
		if !ok || back == 0 {
			continue
		}
		newQty := it.DispensedQty - back
		if newQty < 0 {
			newQty = 0
		}
		if err := tx.Model(&model.PrescriptionItem{}).
			Where("id = ?", it.ID).
			Update("dispensed_qty", newQty).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}
	return nil
}
