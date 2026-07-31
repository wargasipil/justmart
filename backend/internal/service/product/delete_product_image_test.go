package product_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestDeleteProductImage_ClearsBytesAndMarker(t *testing.T) {
	t.Parallel()
	svc, ctx, productID := newImageEnv(t)

	_, err := svc.UploadProductImage(ctx, connect.NewRequest(&inventoryifacev1.UploadProductImageRequest{
		ProductId: productID, ImageData: fakeImage(512), ThumbData: fakeThumb(64), ContentType: "image/jpeg",
	}))
	require.NoError(t, err)

	_, err = svc.DeleteProductImage(ctx, connect.NewRequest(&inventoryifacev1.DeleteProductImageRequest{
		ProductId: productID,
	}))
	require.NoError(t, err)

	got, err := svc.GetProductImage(ctx, connect.NewRequest(&inventoryifacev1.GetProductImageRequest{
		ProductId: productID,
	}))
	require.NoError(t, err)
	require.Empty(t, got.Msg.ImageData)

	// The marker must clear in the same breath — a product row still claiming a
	// picture would make every surface fetch bytes that no longer exist.
	prod, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: productID}))
	require.NoError(t, err)
	require.Zero(t, prod.Msg.Product.ImageUpdatedAt)
}

func TestDeleteProductImage_Idempotent(t *testing.T) {
	t.Parallel()
	svc, ctx, productID := newImageEnv(t)

	// Deleting when there is nothing to delete succeeds.
	_, err := svc.DeleteProductImage(ctx, connect.NewRequest(&inventoryifacev1.DeleteProductImageRequest{
		ProductId: productID,
	}))
	require.NoError(t, err)
}

func TestDeleteProductImage_UnknownProduct(t *testing.T) {
	t.Parallel()
	svc, ctx, _ := newImageEnv(t)

	_, err := svc.DeleteProductImage(ctx, connect.NewRequest(&inventoryifacev1.DeleteProductImageRequest{
		ProductId: "00000000-0000-0000-0000-0000000000ff",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
