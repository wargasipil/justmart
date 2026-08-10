package productpricetier

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/service/common"
)

// UpdateProductPriceTier edits a tier's unit, threshold and price. product_id is
// immutable — a tier belongs to the product it was created under.
func (s *ProductPriceTierService) UpdateProductPriceTier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdateProductPriceTierRequest],
) (*connect.Response[inventoryifacev1.UpdateProductPriceTierResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	t, err := s.load(ctx, msg.Id)
	if err != nil {
		return nil, err
	}
	if err := validateTier(msg.MinQty, msg.Price); err != nil {
		return nil, err
	}
	unit, err := resolveTierUnit(ctx, s.db, t.ProductID, msg.ProductUnitId)
	if err != nil {
		return nil, err
	}
	// Exclude our own row so re-saving an unchanged tier isn't a conflict.
	if err := assertTierFree(s.db.WithContext(ctx), unit.ID, msg.MinQty, t.ID); err != nil {
		return nil, err
	}

	// The rung this tier used to be, captured before Updates mutates the model.
	oldUnitID, oldMinQty, oldPrice := t.ProductUnitID, t.MinQty, t.Price
	movedRung := oldUnitID != unit.ID || oldMinQty != msg.MinQty

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockProduct(tx, t.ProductID); err != nil {
			return err
		}
		if err := tx.Model(t).Updates(map[string]any{
			"product_unit_id": unit.ID,
			"unit_name":       unit.Name,
			"unit_factor":     unit.Factor,
			"min_qty":         msg.MinQty,
			"price":           msg.Price,
		}).Error; err != nil {
			return err
		}
		// Mirror the write onto the model explicitly rather than relying on GORM to
		// fold a map update back into it — the response and the history row below
		// both read these fields.
		t.ProductUnitID, t.UnitName, t.UnitFactor = unit.ID, unit.Name, unit.Factor
		t.MinQty, t.Price = msg.MinQty, msg.Price

		if !movedRung && oldPrice == msg.Price {
			// Nothing priceworthy changed (a re-save, or a unit rename). Writing a
			// version row here would fill the history with duplicate prices —
			// syncProductUnits guards its own writes the same way.
			return nil
		}
		now := time.Now()
		if movedRung {
			// The tier moved: the rung it left stops having a price, and the rung it
			// arrived at starts having one. Two separate facts.
			if err := common.CloseTierRung(tx, oldUnitID, oldMinQty, now); err != nil {
				return err
			}
		}
		return common.RecordTierPrice(tx, t, caller.UserID, now)
	})
	if err != nil {
		return nil, wrapTxError(err)
	}
	return connect.NewResponse(&inventoryifacev1.UpdateProductPriceTierResponse{Tier: toProto(t)}), nil
}
