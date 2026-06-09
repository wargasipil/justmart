package sale_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
)

func TestAddItem_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	productID := seedProduct(t, db, "ai-sku-1", "Paracetamol", 2000)
	saleID := startDraft(t, svc, ctx)

	resp, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId:    saleID,
		ProductId: productID,
		Qty:       3,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Sale)
	require.Len(t, resp.Msg.Sale.Items, 1)
	item := resp.Msg.Sale.Items[0]
	require.Equal(t, productID, item.ProductId)
	require.Equal(t, int32(3), item.Qty)
	require.Equal(t, int64(2000), item.UnitPriceSnapshot)
	require.Equal(t, int64(6000), item.LineTotal)
	require.Equal(t, int64(6000), resp.Msg.Sale.Subtotal)
	require.Equal(t, int64(6000), resp.Msg.Sale.Total)
}

func TestAddItem_MergesSameProduct(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	productID := seedProduct(t, db, "ai-sku-2", "Amoxicillin", 1500)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 2,
	}))
	require.NoError(t, err)
	resp, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 4,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Sale.Items, 1)
	require.Equal(t, int32(6), resp.Msg.Sale.Items[0].Qty)
}

func TestAddItem_ZeroQty(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	productID := seedProduct(t, db, "ai-sku-3", "Vitamin C", 1000)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 0,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAddItem_ProductNotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId:    saleID,
		ProductId: "00000000-0000-0000-0000-0000000000ff",
		Qty:       1,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestAddItem_SaleNotFound(t *testing.T) {
	t.Parallel()
	svc, _, db, _ := newSaleSvc(t)
	productID := seedProduct(t, db, "ai-sku-4", "Ibuprofen", 1200)

	_, err := svc.AddItem(context.Background(), connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId:    "00000000-0000-0000-0000-0000000000ee",
		ProductId: productID,
		Qty:       1,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
