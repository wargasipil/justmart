package stocktake_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
)

func TestSetLineDisposition_WriteOff(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)
	batchID := seedBatchWithStock(t, db, ownerID, whID, 20)
	sessionID := startDraft(t, svc, ctx, "count")
	lineID := addBatch(t, svc, db, ctx, sessionID, batchID)

	resp, err := svc.SetLineDisposition(ctx, connect.NewRequest(&stocktakeifacev1.SetLineDispositionRequest{
		LineId:          lineID,
		Disposition:     "WRITE_OFF",
		WriteOffKind:    "EXPIRED",
		DispositionNote: "past expiry",
	}))
	require.NoError(t, err)
	line := resp.Msg.Line
	require.NotNil(t, line)
	require.Equal(t, "WRITE_OFF", line.Disposition)
	require.Equal(t, "EXPIRED", line.WriteOffKind)
	require.Equal(t, "past expiry", line.DispositionNote)
}

func TestSetLineDisposition_WriteOffRequiresKind(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)
	batchID := seedBatchWithStock(t, db, ownerID, whID, 20)
	sessionID := startDraft(t, svc, ctx, "count")
	lineID := addBatch(t, svc, db, ctx, sessionID, batchID)

	_, err := svc.SetLineDisposition(ctx, connect.NewRequest(&stocktakeifacev1.SetLineDispositionRequest{
		LineId:      lineID,
		Disposition: "WRITE_OFF", // missing write_off_kind
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSetLineDisposition_InvalidDisposition(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)
	batchID := seedBatchWithStock(t, db, ownerID, whID, 20)
	sessionID := startDraft(t, svc, ctx, "count")
	lineID := addBatch(t, svc, db, ctx, sessionID, batchID)

	_, err := svc.SetLineDisposition(ctx, connect.NewRequest(&stocktakeifacev1.SetLineDispositionRequest{
		LineId:      lineID,
		Disposition: "BOGUS",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
