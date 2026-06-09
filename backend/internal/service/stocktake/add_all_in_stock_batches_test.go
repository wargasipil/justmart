package stocktake_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
)

func TestAddAllInStockBatches_SeedsOnlyInStock(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)

	inStock := seedBatchWithStock(t, db, ownerID, whID, 12)
	// A batch with zero net stock in this warehouse must NOT be seeded.
	zeroStock := seedBatchWithStock(t, db, ownerID, whID, 0)

	sessionID := startDraft(t, svc, ctx, "full count")

	resp, err := svc.AddAllInStockBatches(ctx, connect.NewRequest(&stocktakeifacev1.AddAllInStockBatchesRequest{
		SessionId: sessionID,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.AddedCount) // only the in-stock batch
	require.Equal(t, int32(0), resp.Msg.SkippedCount)

	get, err := svc.GetStocktake(ctx, connect.NewRequest(&stocktakeifacev1.GetStocktakeRequest{Id: sessionID}))
	require.NoError(t, err)
	require.Len(t, get.Msg.Lines, 1)
	require.Equal(t, inStock, get.Msg.Lines[0].BatchId)
	require.NotEqual(t, zeroStock, get.Msg.Lines[0].BatchId)
}

func TestAddAllInStockBatches_NotFound(t *testing.T) {
	t.Parallel()
	svc, _, ctx, _ := newSvc(t)

	_, err := svc.AddAllInStockBatches(ctx, connect.NewRequest(&stocktakeifacev1.AddAllInStockBatchesRequest{
		SessionId: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
