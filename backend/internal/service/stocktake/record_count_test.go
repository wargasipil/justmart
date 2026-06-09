package stocktake_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
)

func TestRecordCount_RecordsVariance(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)
	batchID := seedBatchWithStock(t, db, ownerID, whID, 20)
	sessionID := startDraft(t, svc, ctx, "count")
	lineID := addBatch(t, svc, db, ctx, sessionID, batchID)

	resp, err := svc.RecordCount(ctx, connect.NewRequest(&stocktakeifacev1.RecordCountRequest{
		LineId:     lineID,
		CountedQty: 18,
	}))
	require.NoError(t, err)
	line := resp.Msg.Line
	require.NotNil(t, line)
	require.True(t, line.Counted)
	require.Equal(t, int32(18), line.CountedQty)
	require.Equal(t, int32(20), line.ExpectedQty)
	require.Equal(t, int32(-2), line.Variance)
}

func TestRecordCount_NegativeRejected(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)
	batchID := seedBatchWithStock(t, db, ownerID, whID, 20)
	sessionID := startDraft(t, svc, ctx, "count")
	lineID := addBatch(t, svc, db, ctx, sessionID, batchID)

	_, err := svc.RecordCount(ctx, connect.NewRequest(&stocktakeifacev1.RecordCountRequest{
		LineId:     lineID,
		CountedQty: -1,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRecordCount_LineNotFound(t *testing.T) {
	t.Parallel()
	svc, _, ctx, _ := newSvc(t)

	_, err := svc.RecordCount(ctx, connect.NewRequest(&stocktakeifacev1.RecordCountRequest{
		LineId:     "00000000-0000-0000-0000-000000000000",
		CountedQty: 1,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
