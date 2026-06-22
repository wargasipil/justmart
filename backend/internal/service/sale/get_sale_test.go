package sale_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestGetSale_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	productID := seedProduct(t, db, "gs-sku-1", "Paracetamol", 2000)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: 2,
	}))
	require.NoError(t, err)

	resp, err := svc.GetSale(ctx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: saleID}))
	require.NoError(t, err)
	require.Equal(t, saleID, resp.Msg.Sale.Id)
	require.Len(t, resp.Msg.Sale.Items, 1)
	require.Equal(t, productID, resp.Msg.Sale.Items[0].ProductId)
}

func TestGetSale_NotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)

	_, err := svc.GetSale(ctx, connect.NewRequest(&posifacev1.GetSaleRequest{
		Id: "00000000-0000-0000-0000-0000000000a7",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSale_EmptyID(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)

	_, err := svc.GetSale(ctx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: ""}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestGetSale_CashierAllowedOnOwnSale(t *testing.T) {
	t.Parallel()
	svc, _, db, _ := newSaleSvc(t)
	cashierID := seedCashier(t, db, "gs-own@x.test")
	cashierCtx := servicetest.CtxAs(context.Background(), "CASHIER", cashierID)
	saleID := startDraft(t, svc, cashierCtx) // stamps cashier_user_id = cashierID

	resp, err := svc.GetSale(cashierCtx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: saleID}))
	require.NoError(t, err)
	require.Equal(t, saleID, resp.Msg.Sale.Id)
}

func TestGetSale_CashierDeniedOnForeignSale(t *testing.T) {
	t.Parallel()
	svc, ownerCtx, db, _ := newSaleSvc(t)
	ownerSale := startDraft(t, svc, ownerCtx) // owned by the bootstrap owner
	cashierID := seedCashier(t, db, "gs-foreign@x.test")
	cashierCtx := servicetest.CtxAs(context.Background(), "CASHIER", cashierID)

	_, err := svc.GetSale(cashierCtx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: ownerSale}))
	require.Error(t, err)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestGetSale_OwnerAllowedOnAnySale(t *testing.T) {
	t.Parallel()
	svc, ownerCtx, db, _ := newSaleSvc(t)
	cashierID := seedCashier(t, db, "gs-any@x.test")
	cashierCtx := servicetest.CtxAs(context.Background(), "CASHIER", cashierID)
	cashierSale := startDraft(t, svc, cashierCtx)

	resp, err := svc.GetSale(ownerCtx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: cashierSale}))
	require.NoError(t, err)
	require.Equal(t, cashierSale, resp.Msg.Sale.Id)
}

func TestGetSale_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newSaleSvc(t)

	_, err := svc.GetSale(context.Background(), connect.NewRequest(&posifacev1.GetSaleRequest{
		Id: "00000000-0000-0000-0000-0000000000a7",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
