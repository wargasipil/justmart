package productpricetier

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// DeleteProductPriceTier hard-deletes a tier (mirrors DeleteProductDiscount).
// A DRAFT cart line priced by this tier keeps its snapshot until the next qty
// change, at which point it reverts to the normal price — same as a deleted
// product discount or an edited sell_price.
func (s *ProductPriceTierService) DeleteProductPriceTier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.DeleteProductPriceTierRequest],
) (*connect.Response[inventoryifacev1.DeleteProductPriceTierResponse], error) {
	t, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	// The rung stops having a price: close its open history row, open nothing.
	// The history survives the hard delete because it is keyed by the rung, not
	// by this row's id.
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockProduct(tx, t.ProductID); err != nil {
			return err
		}
		if err := tx.Where("id = ?", t.ID).Delete(&model.ProductPriceTier{}).Error; err != nil {
			return err
		}
		return common.CloseTierRung(tx, t.ProductUnitID, t.MinQty, time.Now())
	})
	if err != nil {
		return nil, wrapTxError(err)
	}
	return connect.NewResponse(&inventoryifacev1.DeleteProductPriceTierResponse{}), nil
}
