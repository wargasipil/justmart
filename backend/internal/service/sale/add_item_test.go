package sale_test

import (
	"errors"
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

// requireToken asserts a connect error with the given code whose message is the
// stable token (POS reads sale.not_found / sale.not_draft to recover a dead draft).
func requireToken(t *testing.T, err error, code connect.Code, token string) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, code, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, token, ce.Message())
}

func TestAddItem_SaleNotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	productID := seedProduct(t, db, "ai-sku-4", "Ibuprofen", 1200)

	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId:    "00000000-0000-0000-0000-0000000000ee",
		ProductId: productID,
		Qty:       1,
	}))
	requireToken(t, err, connect.CodeNotFound, "sale.not_found")
}

// The POS bug: the till still holds a draft id the sweeper (or a discard)
// already deleted. Every add then answers sale.not_found, never prose.
func TestAddItem_DiscardedDraft(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	productID := seedProduct(t, db, "ai-sku-6", "Antasida", 900)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 1,
	}))
	require.NoError(t, err)
	_, err = svc.DiscardSale(ctx, connect.NewRequest(&posifacev1.DiscardSaleRequest{SaleId: saleID}))
	require.NoError(t, err)

	_, err = svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 1,
	}))
	requireToken(t, err, connect.CodeNotFound, "sale.not_found")
}

// A COMPLETED sale is frozen: AddItem answers FailedPrecondition + sale.not_draft.
func TestAddItem_CompletedSale(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "ai-sku-5", "Cetirizine", 1000)
	seedStock(t, db, productID, ownerID, 10)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 1,
	}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    1000,
	}))
	require.NoError(t, err)

	_, err = svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 1,
	}))
	requireToken(t, err, connect.CodeFailedPrecondition, "sale.not_draft")
}
