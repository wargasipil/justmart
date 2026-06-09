package sale_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestGetTodaySnapshot_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "ts-sku-1", "Paracetamol", 2000)
	seedStock(t, db, productID, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	completeOne(t, svc, ctx, productID, saleID, 4, 10000)

	resp, err := svc.GetTodaySnapshot(ctx, connect.NewRequest(&posifacev1.GetTodaySnapshotRequest{}))
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.Msg.SaleCount)
	require.Equal(t, int64(8000), resp.Msg.Revenue)   // 4 × 2000
	require.Equal(t, int64(4), resp.Msg.ItemsSold)     // base units
	require.Equal(t, productID, resp.Msg.TopProductId) // best seller
	require.Equal(t, int64(4), resp.Msg.TopProductQty)
	require.NotZero(t, resp.Msg.LastSaleUnix)
}

func TestGetTodaySnapshot_EmptyIsZero(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)

	resp, err := svc.GetTodaySnapshot(ctx, connect.NewRequest(&posifacev1.GetTodaySnapshotRequest{}))
	require.NoError(t, err)
	require.Equal(t, int64(0), resp.Msg.SaleCount)
	require.Equal(t, int64(0), resp.Msg.Revenue)
	require.Equal(t, int64(0), resp.Msg.LastSaleUnix)
}

func TestGetTodaySnapshot_CashierFilterForeignDenied(t *testing.T) {
	t.Parallel()
	svc, _, _, ownerID := newSaleSvc(t)
	// A CASHIER may only request their own snapshot; asking for another user's
	// id is InvalidArgument.
	cashierCtx := servicetest.CtxAs(context.Background(), "CASHIER", ownerID)

	_, err := svc.GetTodaySnapshot(cashierCtx, connect.NewRequest(&posifacev1.GetTodaySnapshotRequest{
		CashierUserId: "00000000-0000-0000-0000-0000000000a6",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestGetTodaySnapshot_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newSaleSvc(t)

	_, err := svc.GetTodaySnapshot(context.Background(), connect.NewRequest(&posifacev1.GetTodaySnapshotRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
