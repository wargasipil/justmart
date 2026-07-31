package product_test

import (
	"bytes"
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	productsvc "github.com/justmart/backend/internal/service/product"
	"github.com/justmart/backend/internal/service/servicetest"
)

// Renditions are opaque bytes to the handler (it never decodes them), so the
// fixtures only need to be distinguishable and correctly sized.
func fakeImage(n int) []byte { return bytes.Repeat([]byte{0xAA}, n) }
func fakeThumb(n int) []byte { return bytes.Repeat([]byte{0xBB}, n) }

// newImageEnv wires the service + an owner principal and seeds one product.
func newImageEnv(t *testing.T) (*productsvc.ProductService, context.Context, string) {
	t.Helper()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	return svc, ctx, seedProduct(t, svc, ctx, "IMG-1", "Paracetamol", 5000)
}

func TestUploadProductImage_StoresBothRenditions(t *testing.T) {
	t.Parallel()
	svc, ctx, productID := newImageEnv(t)

	res, err := svc.UploadProductImage(ctx, connect.NewRequest(&inventoryifacev1.UploadProductImageRequest{
		ProductId:   productID,
		ImageData:   fakeImage(4096),
		ThumbData:   fakeThumb(256),
		ContentType: "image/jpeg",
	}))
	require.NoError(t, err)
	require.NotZero(t, res.Msg.ImageUpdatedAt)

	// Both renditions must come back distinctly — a handler that stored the
	// original in both columns would pass a single-variant test.
	thumb, err := svc.GetProductImage(ctx, connect.NewRequest(&inventoryifacev1.GetProductImageRequest{
		ProductId: productID,
		Variant:   inventoryifacev1.ProductImageVariant_PRODUCT_IMAGE_VARIANT_THUMB,
	}))
	require.NoError(t, err)
	require.Equal(t, fakeThumb(256), thumb.Msg.ImageData)

	orig, err := svc.GetProductImage(ctx, connect.NewRequest(&inventoryifacev1.GetProductImageRequest{
		ProductId: productID,
		Variant:   inventoryifacev1.ProductImageVariant_PRODUCT_IMAGE_VARIANT_ORIGINAL,
	}))
	require.NoError(t, err)
	require.Equal(t, fakeImage(4096), orig.Msg.ImageData)
}

func TestUploadProductImage_ReplacesExisting(t *testing.T) {
	t.Parallel()
	svc, ctx, productID := newImageEnv(t)

	_, err := svc.UploadProductImage(ctx, connect.NewRequest(&inventoryifacev1.UploadProductImageRequest{
		ProductId: productID, ImageData: fakeImage(512), ThumbData: fakeThumb(64), ContentType: "image/png",
	}))
	require.NoError(t, err)

	// One picture per product: a re-upload must upsert, not collide on the PK.
	_, err = svc.UploadProductImage(ctx, connect.NewRequest(&inventoryifacev1.UploadProductImageRequest{
		ProductId: productID, ImageData: fakeImage(1024), ThumbData: fakeThumb(128), ContentType: "image/webp",
	}))
	require.NoError(t, err)

	got, err := svc.GetProductImage(ctx, connect.NewRequest(&inventoryifacev1.GetProductImageRequest{
		ProductId: productID,
	}))
	require.NoError(t, err)
	require.Equal(t, fakeThumb(128), got.Msg.ImageData)
	require.Equal(t, "image/webp", got.Msg.ContentType)
}

func TestUploadProductImage_ThumbRequired(t *testing.T) {
	t.Parallel()
	svc, ctx, productID := newImageEnv(t)

	// The two-rendition rule is enforced server-side: an original-only upload
	// is rejected rather than silently leaving list surfaces on heavy bytes.
	_, err := svc.UploadProductImage(ctx, connect.NewRequest(&inventoryifacev1.UploadProductImageRequest{
		ProductId: productID, ImageData: fakeImage(512), ContentType: "image/jpeg",
	}))
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, connect.CodeInvalidArgument, ce.Code())
	require.Equal(t, "product_image.thumb_required", ce.Message())
}

func TestUploadProductImage_RejectsOversizeAndBadType(t *testing.T) {
	t.Parallel()
	svc, ctx, productID := newImageEnv(t)

	cases := []struct {
		name  string
		req   *inventoryifacev1.UploadProductImageRequest
		token string
	}{
		{
			name:  "image too large",
			req:   &inventoryifacev1.UploadProductImageRequest{ProductId: productID, ImageData: fakeImage(productsvc.MaxProductImageBytes + 1), ThumbData: fakeThumb(64), ContentType: "image/jpeg"},
			token: "product_image.image_too_large",
		},
		{
			name:  "thumb too large",
			req:   &inventoryifacev1.UploadProductImageRequest{ProductId: productID, ImageData: fakeImage(512), ThumbData: fakeThumb(productsvc.MaxProductImageThumbBytes + 1), ContentType: "image/jpeg"},
			token: "product_image.thumb_too_large",
		},
		{
			name:  "svg rejected",
			req:   &inventoryifacev1.UploadProductImageRequest{ProductId: productID, ImageData: fakeImage(512), ThumbData: fakeThumb(64), ContentType: "image/svg+xml"},
			token: "product_image.content_type_invalid",
		},
		{
			name:  "empty image",
			req:   &inventoryifacev1.UploadProductImageRequest{ProductId: productID, ThumbData: fakeThumb(64), ContentType: "image/jpeg"},
			token: "product_image.image_required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UploadProductImage(ctx, connect.NewRequest(tc.req))
			require.Error(t, err)
			var ce *connect.Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, connect.CodeInvalidArgument, ce.Code())
			require.Equal(t, tc.token, ce.Message())
		})
	}
}

func TestUploadProductImage_UnknownProduct(t *testing.T) {
	t.Parallel()
	svc, ctx, _ := newImageEnv(t)

	_, err := svc.UploadProductImage(ctx, connect.NewRequest(&inventoryifacev1.UploadProductImageRequest{
		ProductId:   "00000000-0000-0000-0000-0000000000ff",
		ImageData:   fakeImage(512),
		ThumbData:   fakeThumb(64),
		ContentType: "image/jpeg",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestUploadProductImage_MarksProductRow(t *testing.T) {
	t.Parallel()
	svc, ctx, productID := newImageEnv(t)

	_, err := svc.UploadProductImage(ctx, connect.NewRequest(&inventoryifacev1.UploadProductImageRequest{
		ProductId: productID, ImageData: fakeImage(512), ThumbData: fakeThumb(64), ContentType: "image/jpeg",
	}))
	require.NoError(t, err)

	// The denormalized marker is what every UI surface reads to decide whether
	// to fetch bytes — so it has to ride along on the LIST read, not just Get.
	list, err := svc.ListProducts(ctx, connect.NewRequest(&inventoryifacev1.ListProductsRequest{}))
	require.NoError(t, err)
	var found bool
	for _, p := range list.Msg.Products {
		if p.Id == productID {
			found = true
			require.NotZero(t, p.ImageUpdatedAt)
		}
	}
	require.True(t, found, "seeded product must appear in ListProducts")

	got, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: productID}))
	require.NoError(t, err)
	require.NotZero(t, got.Msg.Product.ImageUpdatedAt)
}
