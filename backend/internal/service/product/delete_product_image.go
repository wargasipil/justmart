package product

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// DeleteProductImage removes a product's picture, reverting it to the
// placeholder. Idempotent: deleting when there is nothing to delete succeeds.
func (s *ProductService) DeleteProductImage(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.DeleteProductImageRequest],
) (*connect.Response[inventoryifacev1.DeleteProductImageResponse], error) {
	prod, err := s.load(ctx, req.Msg.ProductId)
	if err != nil {
		return nil, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("product_id = ?", prod.ID).
			Delete(&model.ProductImage{}).Error; err != nil {
			return err
		}
		// Clear the denormalized marker in the same tx, so `products` can never
		// claim a picture that the bytes table no longer has.
		return tx.Model(&model.Product{}).
			Where("id = ?", prod.ID).
			Update("image_updated_at", nil).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	return connect.NewResponse(&inventoryifacev1.DeleteProductImageResponse{}), nil
}
