package sale_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestGetSalesSummary_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "gss-sku-1", "Paracetamol", 2000)
	seedStock(t, db, productID, ownerID, 50)

	// Two completed sales: 2 units (4000) + 3 units (6000) = 10000 revenue,
	// 5 base units sold, 2 sales.
	s1 := startDraft(t, svc, ctx)
	completeOne(t, svc, ctx, productID, s1, 2, 10000)
	s2 := startDraft(t, svc, ctx)
	completeOne(t, svc, ctx, productID, s2, 3, 10000)

	resp, err := svc.GetSalesSummary(ctx, connect.NewRequest(&posifacev1.GetSalesSummaryRequest{}))
	require.NoError(t, err)
	require.Equal(t, int64(2), resp.Msg.SaleCount)
	require.Equal(t, int64(5), resp.Msg.ItemsSold)
	require.Equal(t, int64(10000), resp.Msg.Revenue)
}

func TestGetSalesSummary_EmptyIsZero(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)

	resp, err := svc.GetSalesSummary(ctx, connect.NewRequest(&posifacev1.GetSalesSummaryRequest{}))
	require.NoError(t, err)
	require.Equal(t, int64(0), resp.Msg.SaleCount)
	require.Equal(t, int64(0), resp.Msg.ItemsSold)
	require.Equal(t, int64(0), resp.Msg.Revenue)
}

func TestGetSalesSummary_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newSaleSvc(t)

	_, err := svc.GetSalesSummary(context.Background(), connect.NewRequest(&posifacev1.GetSalesSummaryRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestGetSalesSummary_CashierSelfScoped(t *testing.T) {
	t.Parallel()
	svc, ownerCtx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "gss-scope-sku", "Paracetamol", 2000)
	seedStock(t, db, productID, ownerID, 100)
	// Owner: 2 units (4000). Cashier: 3 units (6000).
	ownerSale := startDraft(t, svc, ownerCtx)
	completeOne(t, svc, ownerCtx, productID, ownerSale, 2, 10000)
	cashierID := seedCashier(t, db, "gss-cashier@x.test")
	cashierCtx := servicetest.CtxAs(context.Background(), "CASHIER", cashierID)
	cashierSale := startDraft(t, svc, cashierCtx)
	completeOne(t, svc, cashierCtx, productID, cashierSale, 3, 10000)

	// Cashier summary is self-scoped (proves both the count/revenue scope AND
	// the items_sold subquery scope).
	cashierResp, err := svc.GetSalesSummary(cashierCtx, connect.NewRequest(&posifacev1.GetSalesSummaryRequest{}))
	require.NoError(t, err)
	require.Equal(t, int64(1), cashierResp.Msg.SaleCount)
	require.Equal(t, int64(3), cashierResp.Msg.ItemsSold)
	require.Equal(t, int64(6000), cashierResp.Msg.Revenue)

	// Owner sees the full picture.
	ownerResp, err := svc.GetSalesSummary(ownerCtx, connect.NewRequest(&posifacev1.GetSalesSummaryRequest{}))
	require.NoError(t, err)
	require.Equal(t, int64(2), ownerResp.Msg.SaleCount)
	require.Equal(t, int64(5), ownerResp.Msg.ItemsSold)
	require.Equal(t, int64(10000), ownerResp.Msg.Revenue)

	// Owner filtering to the cashier matches the cashier's own view.
	filtered, err := svc.GetSalesSummary(ownerCtx, connect.NewRequest(&posifacev1.GetSalesSummaryRequest{
		CashierUserId: cashierID,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(1), filtered.Msg.SaleCount)
	require.Equal(t, int64(3), filtered.Msg.ItemsSold)
	require.Equal(t, int64(6000), filtered.Msg.Revenue)
}
