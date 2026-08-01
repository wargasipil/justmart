package product_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestGetProductImage_DefaultsToThumb(t *testing.T) {
	t.Parallel()
	svc, ctx, productID := newImageEnv(t)

	_, err := svc.UploadProductImage(ctx, connect.NewRequest(&inventoryifacev1.UploadProductImageRequest{
		ProductId: productID, ImageData: fakeImage(4096), ThumbData: fakeThumb(256), ContentType: "image/jpeg",
	}))
	require.NoError(t, err)

	// Unset variant must resolve to THUMB — the fast path is the default, and
	// shipping the heavy original is an explicit opt-in.
	got, err := svc.GetProductImage(ctx, connect.NewRequest(&inventoryifacev1.GetProductImageRequest{
		ProductId: productID,
	}))
	require.NoError(t, err)
	require.Equal(t, fakeThumb(256), got.Msg.ImageData)
	require.Equal(t,
		inventoryifacev1.ProductImageVariant_PRODUCT_IMAGE_VARIANT_THUMB,
		got.Msg.Variant)
	require.NotZero(t, got.Msg.ImageUpdatedAt)
}

func TestGetProductImage_NoImageIsNotAnError(t *testing.T) {
	t.Parallel()
	svc, ctx, productID := newImageEnv(t)

	// Absence must come back empty, not NotFound: the UI falls through to its
	// placeholder without special-casing an error.
	got, err := svc.GetProductImage(ctx, connect.NewRequest(&inventoryifacev1.GetProductImageRequest{
		ProductId: productID,
	}))
	require.NoError(t, err)
	require.Empty(t, got.Msg.ImageData)
	require.Zero(t, got.Msg.ImageUpdatedAt)
}

func TestGetProductImage_RequiresProductID(t *testing.T) {
	t.Parallel()
	svc, ctx, _ := newImageEnv(t)

	_, err := svc.GetProductImage(ctx, connect.NewRequest(&inventoryifacev1.GetProductImageRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
