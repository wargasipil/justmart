package product

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// UploadProductImage replaces a product's picture. One picture per product, so
// this is an upsert rather than an insert — re-uploading swaps both renditions.
//
// Both renditions are required: a row with only an original would silently push
// the products list and the POS search rows onto the heavy bytes, which is
// exactly what the two-rendition rule exists to prevent.
func (s *ProductService) UploadProductImage(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UploadProductImageRequest],
) (*connect.Response[inventoryifacev1.UploadProductImageResponse], error) {
	// load() 404s on an unknown id, so an upload can never orphan bytes against
	// a product that isn't there.
	prod, err := s.load(ctx, req.Msg.ProductId)
	if err != nil {
		return nil, err
	}

	image := req.Msg.ImageData
	thumb := req.Msg.ThumbData
	if len(image) == 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "product_image.image_required")
	}
	if len(thumb) == 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "product_image.thumb_required")
	}
	if len(image) > MaxProductImageBytes {
		return nil, common.TokenError(connect.CodeInvalidArgument, "product_image.image_too_large")
	}
	if len(thumb) > MaxProductImageThumbBytes {
		return nil, common.TokenError(connect.CodeInvalidArgument, "product_image.thumb_too_large")
	}
	if !allowedProductImageTypes[req.Msg.ContentType] {
		return nil, common.TokenError(connect.CodeInvalidArgument, "product_image.content_type_invalid")
	}

	now := time.Now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := model.ProductImage{
			ProductID:   prod.ID,
			ContentType: req.Msg.ContentType,
			ImageData:   image,
			ThumbData:   thumb,
			UpdatedAt:   now,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "product_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"content_type", "image_data", "thumb_data", "updated_at",
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
		// Keep the denormalized marker on `products` in step, in the same tx —
		// it is what every other read uses to decide whether a picture exists.
		return tx.Model(&model.Product{}).
			Where("id = ?", prod.ID).
			Update("image_updated_at", now).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	return connect.NewResponse(&inventoryifacev1.UploadProductImageResponse{
		ImageUpdatedAt: now.Unix(),
	}), nil
}
