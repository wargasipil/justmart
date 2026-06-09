package stocktake_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
)

func TestAddBatchesToSession_AddsAndDedupes(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)
	batchID := seedBatchWithStock(t, db, ownerID, whID, 20)
	sessionID := startDraft(t, svc, ctx, "count")

	// First add inserts a line with expected_qty snapshotted from current stock.
	resp, err := svc.AddBatchesToSession(ctx, connect.NewRequest(&stocktakeifacev1.AddBatchesToSessionRequest{
		SessionId: sessionID,
		BatchIds:  []string{batchID},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.AddedCount)
	require.Equal(t, int32(0), resp.Msg.SkippedCount)

	// Re-adding the same batch is a silent no-op (ON CONFLICT DO NOTHING).
	resp2, err := svc.AddBatchesToSession(ctx, connect.NewRequest(&stocktakeifacev1.AddBatchesToSessionRequest{
		SessionId: sessionID,
		BatchIds:  []string{batchID},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(0), resp2.Msg.AddedCount)
	require.Equal(t, int32(1), resp2.Msg.SkippedCount)

	// Verify the snapshotted expected_qty matches seeded stock.
	get, err := svc.GetStocktake(ctx, connect.NewRequest(&stocktakeifacev1.GetStocktakeRequest{Id: sessionID}))
	require.NoError(t, err)
	require.Len(t, get.Msg.Lines, 1)
	require.Equal(t, int32(20), get.Msg.Lines[0].ExpectedQty)
}

func TestAddBatchesToSession_NotFound(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)
	batchID := seedBatchWithStock(t, db, ownerID, whID, 5)

	_, err := svc.AddBatchesToSession(ctx, connect.NewRequest(&stocktakeifacev1.AddBatchesToSessionRequest{
		SessionId: "00000000-0000-0000-0000-000000000000",
		BatchIds:  []string{batchID},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
