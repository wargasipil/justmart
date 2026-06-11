package sale

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
	prescriptionsvc "github.com/justmart/backend/internal/service/prescription"
)

// assertRxCovers enforces the prescription gate for a single product in a DRAFT
// cart. For a product whose `prescription_required` is false it is a no-op. For
// an Rx-required product it requires the sale to have an ACTIVE prescription
// attached that includes the product with enough remaining qty (base units) to
// cover the cart's CURRENT total for that product.
//
// Callers invoke this AFTER persisting the line change inside the tx, so the
// cart SUM reflects the new state; a violation returns FailedPrecondition and
// the surrounding tx rolls the change back.
//
// Prescription enforcement is a PHARMACY-mode concept: in retail mode it is a
// no-op (a product's prescription_required flag is meaningless there), so a
// retail POS is never blocked even if a product carries the flag.
func (s *SaleService) assertRxCovers(ctx context.Context, tx *gorm.DB, sale *model.Sale, productID string) error {
	pharmacy, err := common.IsPharmacyMode(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !pharmacy {
		return nil // retail mode — no Rx gate
	}

	var prod model.Product
	if err := tx.Select("id", "prescription_required").
		Where("id = ?", productID).First(&prod).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !prod.PrescriptionRequired {
		return nil // not an Rx product — no constraint
	}

	if sale.PrescriptionID == nil || *sale.PrescriptionID == "" {
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("this product requires a prescription; attach one to the sale first"))
	}

	var rx model.Prescription
	if err := tx.Preload("Items").Where("id = ?", *sale.PrescriptionID).First(&rx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("attached prescription not found"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}
	if prescriptionsvc.ComputeStatus(&rx, time.Now()) != prescriptionsvc.StatusActive {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("attached prescription is not active"))
	}

	remaining := int32(-1)
	for i := range rx.Items {
		if rx.Items[i].ProductID == productID {
			remaining = rx.Items[i].PrescribedQty - rx.Items[i].DispensedQty
			break
		}
	}
	if remaining < 0 {
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("attached prescription does not cover this product"))
	}

	// Cart total for this product (base units) after the in-tx change.
	var cartBase int64
	if err := tx.Model(&model.SaleItem{}).
		Where("sale_id = ? AND product_id = ?", sale.ID, productID).
		Select("COALESCE(SUM(base_qty), 0)").Scan(&cartBase).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if cartBase > int64(remaining) {
		return connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("prescription covers only %d more of this product, cart has %d", remaining, cartBase))
	}
	return nil
}

// incrementRxDispensed bumps prescription_items.dispensed_qty for every cart
// product that appears on the sale's attached prescription, by the cart's base
// qty for that product. Runs inside CompleteSale's tx after FEFO succeeds.
//
// It locks the prescription row FOR UPDATE first and re-validates remaining qty
// (the cart was last checked at add-time; a concurrent sale could have dispensed
// against the same Rx in between) — a shortfall fails the whole CompleteSale.
func (s *SaleService) incrementRxDispensed(tx *gorm.DB, sale *model.Sale, items []model.SaleItem) error {
	if sale.PrescriptionID == nil || *sale.PrescriptionID == "" {
		return nil
	}

	// Sum base qty per product across the cart.
	perProduct := map[string]int32{}
	for i := range items {
		base := items[i].BaseQty
		if base <= 0 {
			base = items[i].Qty // back-compat for pre-UOM rows
		}
		perProduct[items[i].ProductID] += base
	}

	// Lock the Rx so concurrent completes against the same script serialize.
	var rx model.Prescription
	if err := common.RowLock(tx).Preload("Items").
		Where("id = ?", *sale.PrescriptionID).First(&rx).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for i := range rx.Items {
		it := &rx.Items[i]
		want, ok := perProduct[it.ProductID]
		if !ok || want == 0 {
			continue
		}
		if it.DispensedQty+want > it.PrescribedQty {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("prescription no longer covers product %s (remaining %d, need %d)",
					it.ProductID, it.PrescribedQty-it.DispensedQty, want))
		}
		if err := tx.Model(&model.PrescriptionItem{}).
			Where("id = ?", it.ID).
			Update("dispensed_qty", gorm.Expr("dispensed_qty + ?", want)).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}
	return nil
}
