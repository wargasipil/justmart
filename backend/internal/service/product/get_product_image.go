package product

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// GetProductImage returns ONE rendition of a product's picture. Open to every
// catalog reader (POS renders the thumbnail in its search rows) — it exposes
// nothing a caller can't already see in the catalog.
//
// A product with no picture is NOT an error: the response comes back empty with
// image_updated_at = 0 so the UI falls through to its placeholder without
// special-casing a NotFound.
func (s *ProductService) GetProductImage(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetProductImageRequest],
) (*connect.Response[inventoryifacev1.GetProductImageResponse], error) {
	if req.Msg.ProductId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_id required"))
	}
	variant := resolveImageVariant(req.Msg.Variant)

	// Select only the rendition asked for — fetching both would defeat the
	// point of storing a thumbnail.
	column := "thumb_data"
	if variant == inventoryifacev1.ProductImageVariant_PRODUCT_IMAGE_VARIANT_ORIGINAL {
		column = "image_data"
	}

	var row model.ProductImage
	err := s.db.WithContext(ctx).
		Select(column+" AS image_data", "content_type", "product_id", "updated_at").
		Where("product_id = ?", req.Msg.ProductId).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return connect.NewResponse(&inventoryifacev1.GetProductImageResponse{Variant: variant}), nil
	}
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	return connect.NewResponse(&inventoryifacev1.GetProductImageResponse{
		ImageData:      row.ImageData,
		ContentType:    row.ContentType,
		ImageUpdatedAt: row.UpdatedAt.Unix(),
		Variant:        variant,
	}), nil
}
