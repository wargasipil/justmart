package stocktake_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
)

func TestGetStocktake_ReturnsSessionWithLines(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)
	batchID := seedBatchWithStock(t, db, ownerID, whID, 15)
	sessionID := startDraft(t, svc, ctx, "count")
	addBatch(t, svc, db, ctx, sessionID, batchID)

	resp, err := svc.GetStocktake(ctx, connect.NewRequest(&stocktakeifacev1.GetStocktakeRequest{
		Id: sessionID,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Session)
	require.Equal(t, sessionID, resp.Msg.Session.Id)
	require.Len(t, resp.Msg.Lines, 1)
	line := resp.Msg.Lines[0]
	require.Equal(t, batchID, line.BatchId)
	require.Equal(t, int32(15), line.ExpectedQty)
	// Denormalized batch/product context is hydrated for display.
	require.Equal(t, "Paracetamol", line.ProductName)
	require.NotEmpty(t, line.BatchNumber)
}

func TestGetStocktake_NotFound(t *testing.T) {
	t.Parallel()
	svc, _, ctx, _ := newSvc(t)

	_, err := svc.GetStocktake(ctx, connect.NewRequest(&stocktakeifacev1.GetStocktakeRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
