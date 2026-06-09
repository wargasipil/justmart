package stocktake_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
)

func TestRemoveLine_DeletesLine(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)
	batchID := seedBatchWithStock(t, db, ownerID, whID, 20)
	sessionID := startDraft(t, svc, ctx, "count")
	lineID := addBatch(t, svc, db, ctx, sessionID, batchID)

	_, err := svc.RemoveLine(ctx, connect.NewRequest(&stocktakeifacev1.RemoveLineRequest{
		LineId: lineID,
	}))
	require.NoError(t, err)

	// The session now has no lines.
	get, err := svc.GetStocktake(ctx, connect.NewRequest(&stocktakeifacev1.GetStocktakeRequest{Id: sessionID}))
	require.NoError(t, err)
	require.Empty(t, get.Msg.Lines)
}

func TestRemoveLine_NotFound(t *testing.T) {
	t.Parallel()
	svc, _, ctx, _ := newSvc(t)

	_, err := svc.RemoveLine(ctx, connect.NewRequest(&stocktakeifacev1.RemoveLineRequest{
		LineId: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
