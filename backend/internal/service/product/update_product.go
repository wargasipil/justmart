package product

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *ProductService) UpdateProduct(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdateProductRequest],
) (*connect.Response[inventoryifacev1.UpdateProductResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	med, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Msg.Name)
	unit := strings.TrimSpace(req.Msg.Unit)
	if name == "" || unit == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name and unit required"))
	}
	if req.Msg.UnitPrice < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unit_price must be >= 0"))
	}

	priceChanged := req.Msg.UnitPrice != med.UnitPrice

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the product row so concurrent price edits serialize — otherwise the
		// close-open-row + insert-new-open-row price-version sequence can collide
		// on the *_open_idx partial unique index and fail spuriously.
		if err := common.RowLock(tx).
			Where("id = ?", med.ID).First(&model.Product{}).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"name":                  name,
			"unit":                  unit,
			"prescription_required": req.Msg.PrescriptionRequired,
		}
		if priceChanged {
			updates["unit_price"] = req.Msg.UnitPrice
		}
		if err := tx.Model(med).Updates(updates).Error; err != nil {
			return err
		}

		if priceChanged {
			now := time.Now()
			// Close the current open price row.
			if err := tx.Model(&model.ProductPrice{}).
				Where("product_id = ? AND effective_to IS NULL", med.ID).
				Update("effective_to", now).Error; err != nil {
				return fmt.Errorf("close current price: %w", err)
			}
			// Insert the new open row.
			newPrice := model.ProductPrice{
				ProductID:     med.ID,
				UnitPrice:     req.Msg.UnitPrice,
				EffectiveFrom: now,
				ChangedBy:     caller.UserID,
			}
			if err := tx.Create(&newPrice).Error; err != nil {
				return fmt.Errorf("insert new price: %w", err)
			}
		}

		// Sync units against the new base name/price.
		med.Unit = unit
		med.UnitPrice = req.Msg.UnitPrice
		if err := syncProductUnits(tx, med, req.Msg.Units, caller.UserID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		var ce *connect.Error
		if errors.As(err, &ce) {
			return nil, err
		}
		return nil, connect.NewError(connect.CodeAborted, err)
	}

	// Refresh from DB so response reflects the new state.
	med, err = s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	out := productToProto(med)
	if err := s.attachUnits(ctx, []*inventoryifacev1.Product{out}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.UpdateProductResponse{Product: out}), nil
}
