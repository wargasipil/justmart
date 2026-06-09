package stocktake_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func TestCompleteStocktake_WritesVarianceMovement(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)
	batchID := seedBatchWithStock(t, db, ownerID, whID, 20)
	sessionID := startDraft(t, svc, ctx, "count")
	lineID := addBatch(t, svc, db, ctx, sessionID, batchID)

	// Count short by 2 (default ADJUSTMENT disposition).
	_, err := svc.RecordCount(ctx, connect.NewRequest(&stocktakeifacev1.RecordCountRequest{
		LineId:     lineID,
		CountedQty: 18,
	}))
	require.NoError(t, err)

	resp, err := svc.CompleteStocktake(ctx, connect.NewRequest(&stocktakeifacev1.CompleteStocktakeRequest{
		SessionId: sessionID,
	}))
	require.NoError(t, err)
	require.Equal(t, "COMPLETED", resp.Msg.Session.Status)
	require.Equal(t, int32(1), resp.Msg.MovementsWritten)
	require.NotZero(t, resp.Msg.Session.CompletedAt)

	// One ADJUSTMENT movement of -2 was written and linked to the line.
	var mv model.StockMovement
	require.NoError(t, db.Where("stocktake_line_id = ?", lineID).First(&mv).Error)
	require.Equal(t, int32(-2), mv.Qty)
	require.Equal(t, "ADJUSTMENT", mv.Type)
	require.Equal(t, whID, mv.WarehouseID)
}

func TestCompleteStocktake_PositiveVarianceWriteOffRejected(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)
	batchID := seedBatchWithStock(t, db, ownerID, whID, 20)
	sessionID := startDraft(t, svc, ctx, "count")
	lineID := addBatch(t, svc, db, ctx, sessionID, batchID)

	// Count higher than expected (positive variance)...
	_, err := svc.RecordCount(ctx, connect.NewRequest(&stocktakeifacev1.RecordCountRequest{
		LineId:     lineID,
		CountedQty: 25,
	}))
	require.NoError(t, err)
	// ...but mark it WRITE_OFF, which is invalid for a positive variance.
	_, err = svc.SetLineDisposition(ctx, connect.NewRequest(&stocktakeifacev1.SetLineDispositionRequest{
		LineId:       lineID,
		Disposition:  "WRITE_OFF",
		WriteOffKind: "DAMAGED",
	}))
	require.NoError(t, err)

	_, err = svc.CompleteStocktake(ctx, connect.NewRequest(&stocktakeifacev1.CompleteStocktakeRequest{
		SessionId: sessionID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	// Session stays DRAFT (whole tx rolled back).
	get, err := svc.GetStocktake(ctx, connect.NewRequest(&stocktakeifacev1.GetStocktakeRequest{Id: sessionID}))
	require.NoError(t, err)
	require.Equal(t, "DRAFT", get.Msg.Session.Status)
}

func TestCompleteStocktake_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc, _, ctx, _ := newSvc(t)
	sessionID := startDraft(t, svc, ctx, "count")

	_, err := svc.CompleteStocktake(context.Background(), connect.NewRequest(&stocktakeifacev1.CompleteStocktakeRequest{
		SessionId: sessionID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
