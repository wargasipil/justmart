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
	sku := strings.TrimSpace(req.Msg.Sku)
	if name == "" || unit == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "product.required")
	}
	if req.Msg.UnitPrice < 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "product.unit_price_negative")
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
		// Switching AWAY from COMPOSITE while recipe lines still exist is refused:
		// the rows would survive but stop being consulted, so the menu item would
		// go on selling while silently deducting nothing from its ingredients.
		// Clearing the recipe first makes that an explicit act.
		newKind := kindFromProto(req.Msg.Kind)
		if common.NormalizeProductKind(med.Kind) == common.ProductKindComposite &&
			newKind != common.ProductKindComposite {
			var lines int64
			if err := tx.Model(&model.ProductRecipeItem{}).
				Where("product_id = ?", med.ID).Count(&lines).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			if lines > 0 {
				return common.TokenError(connect.CodeFailedPrecondition, "product.kind_has_recipe")
			}
		}

		updates := map[string]any{
			"name":                  name,
			"unit":                  unit,
			"prescription_required": req.Msg.PrescriptionRequired,
			"product_kind":          newKind,
		}
		// SKU is an editable unique business code. Apply only when a (changed)
		// value is provided — empty keeps the current SKU (partial-update safe).
		// Pre-check uniqueness excluding this row (products.sku unique index
		// backstops the race).
		if sku != "" && sku != med.SKU {
			if taken, e := common.ExistsBy(tx, &model.Product{}, "sku = ? AND id <> ?", sku, med.ID); e != nil {
				return connect.NewError(connect.CodeInternal, e)
			} else if taken {
				return common.TokenError(connect.CodeAlreadyExists, "product.sku_taken")
			}
			updates["sku"] = sku
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
