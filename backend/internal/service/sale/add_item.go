package sale

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SaleService) AddItem(
	ctx context.Context,
	req *connect.Request[posifacev1.AddItemRequest],
) (*connect.Response[posifacev1.AddItemResponse], error) {
	if req.Msg.Qty <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("qty must be > 0"))
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}

		// Look up product to snapshot the current price.
		var med model.Product
		if err := tx.Where("id = ?", req.Msg.ProductId).First(&med).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, fmt.Errorf("product %s not found", req.Msg.ProductId))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if !med.Active {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("product is archived"))
		}

		// Resolve the selling unit (box/strip/tablet, …) — default base.
		unit, err := resolveSellUnit(tx, med.ID, req.Msg.ProductUnitId)
		if err != nil {
			return err
		}

		// Look up the existing line for this product + SAME unit.
		var existing model.SaleItem
		findErr := tx.Where("sale_id = ? AND product_id = ? AND product_unit_id = ?", sale.ID, med.ID, unit.ID).
			First(&existing).Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return connect.NewError(connect.CodeInternal, findErr)
		}
		newUnitQty := req.Msg.Qty
		if findErr == nil {
			newUnitQty = existing.Qty + req.Msg.Qty
		}
		newBaseQty := newUnitQty * int32(unit.Factor)

		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			unitID := unit.ID
			item := model.SaleItem{
				SaleID:            sale.ID,
				ProductID:         med.ID,
				ProductUnitID:     &unitID,
				UnitName:          unit.Name,
				UnitFactor:        unit.Factor,
				Qty:               req.Msg.Qty,
				BaseQty:           req.Msg.Qty * int32(unit.Factor),
				UnitPriceSnapshot: unit.SellPrice,
			}
			item.LineTotal = computeLineTotal(item.Qty, item.UnitPriceSnapshot, item.LineDiscount)
			if err := tx.Create(&item).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		} else {
			existing.Qty = newUnitQty
			existing.BaseQty = newBaseQty
			existing.UnitName = unit.Name
			existing.UnitFactor = unit.Factor
			existing.UnitPriceSnapshot = unit.SellPrice
			existing.LineTotal = computeLineTotal(existing.Qty, existing.UnitPriceSnapshot, existing.LineDiscount)
			if err := tx.Save(&existing).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		if err := recomputeSaleTotals(tx, sale.ID); err != nil {
			return err
		}
		// Pharmacy gate: an Rx-required product needs a covering ACTIVE Rx.
		// No-op in retail mode (see assertRxCovers).
		return s.assertRxCovers(ctx, tx, sale, med.ID)
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.AddItemResponse{Sale: saleToProto(sale)}), nil
}
