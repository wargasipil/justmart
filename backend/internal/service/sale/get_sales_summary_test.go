package sale_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
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
