package stocktake_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
	stocktakesvc "github.com/justmart/backend/internal/service/stocktake"
)

func summary(t *testing.T, svc *stocktakesvc.StocktakeService, ctx context.Context) *stocktakeifacev1.GetStocktakeSummaryResponse {
	t.Helper()
	resp, err := svc.GetStocktakeSummary(ctx, connect.NewRequest(&stocktakeifacev1.GetStocktakeSummaryRequest{}))
	require.NoError(t, err)
	return resp.Msg
}

func TestGetStocktakeSummary_EmptyWarehouse(t *testing.T) {
	t.Parallel()
	svc, _, ctx, _ := newSvc(t)

	msg := summary(t, svc, ctx)
	require.Equal(t, int32(0), msg.DraftCount)
	require.Equal(t, int32(0), msg.CompletedCount)
	require.Equal(t, int32(0), msg.VoidedCount)
	require.Equal(t, int32(0), msg.VarianceLines)
	require.Equal(t, int64(0), msg.LastCompletedAt)
}

func TestGetStocktakeSummary_CountsByStatus(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)

	// A completed session carrying one variance line and one matching line.
	completedID := startDraft(t, svc, ctx, "completed")
	shortBatch := seedBatchWithStock(t, db, ownerID, whID, 10)
	exactBatch := seedBatchWithStock(t, db, ownerID, whID, 5)
	shortLine := addBatch(t, svc, db, ctx, completedID, shortBatch)
	exactLine := addBatch(t, svc, db, ctx, completedID, exactBatch)
	recordCount(t, svc, ctx, shortLine, 8) // variance -2
	recordCount(t, svc, ctx, exactLine, 5) // no variance
	_, err := svc.CompleteStocktake(ctx, connect.NewRequest(&stocktakeifacev1.CompleteStocktakeRequest{
		SessionId: completedID,
	}))
	require.NoError(t, err)

	// A voided session.
	voidedID := startDraft(t, svc, ctx, "voided")
	_, err = svc.VoidStocktake(ctx, connect.NewRequest(&stocktakeifacev1.VoidStocktakeRequest{SessionId: voidedID}))
	require.NoError(t, err)

	// An open draft whose variance must NOT count — no movements were booked.
	draftID := startDraft(t, svc, ctx, "draft")
	draftBatch := seedBatchWithStock(t, db, ownerID, whID, 7)
	recordCount(t, svc, ctx, addBatch(t, svc, db, ctx, draftID, draftBatch), 3)

	msg := summary(t, svc, ctx)
	require.Equal(t, int32(1), msg.DraftCount)
	require.Equal(t, int32(1), msg.CompletedCount)
	require.Equal(t, int32(1), msg.VoidedCount)
	require.Equal(t, int32(1), msg.VarianceLines, "only the completed session's short line counts")
	require.Greater(t, msg.LastCompletedAt, int64(0))
}

func TestGetStocktakeSummary_HonorsDateRange(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)

	// One completed session with a variance, backdated 90 days.
	oldID := startDraft(t, svc, ctx, "old")
	batch := seedBatchWithStock(t, db, ownerID, whID, 10)
	recordCount(t, svc, ctx, addBatch(t, svc, db, ctx, oldID, batch), 8)
	_, err := svc.CompleteStocktake(ctx, connect.NewRequest(&stocktakeifacev1.CompleteStocktakeRequest{
		SessionId: oldID,
	}))
	require.NoError(t, err)
	old := time.Now().AddDate(0, 0, -90)
	backdateSession(t, db, oldID, old, old)

	// A fresh draft inside the window.
	startDraft(t, svc, ctx, "recent")

	// Unbounded: both sessions, and the old variance line.
	all := summary(t, svc, ctx)
	require.Equal(t, int32(1), all.CompletedCount)
	require.Equal(t, int32(1), all.DraftCount)
	require.Equal(t, int32(1), all.VarianceLines)
	require.Greater(t, all.LastCompletedAt, int64(0))

	// Last 30 days: the old session and its variance drop out, and with no
	// completed session left in range LastCompletedAt falls back to "never".
	ranged, err := svc.GetStocktakeSummary(ctx, connect.NewRequest(&stocktakeifacev1.GetStocktakeSummaryRequest{
		FromUnix: time.Now().AddDate(0, 0, -30).Unix(),
		ToUnix:   time.Now().Add(time.Hour).Unix(),
	}))
	require.NoError(t, err)
	require.Equal(t, int32(0), ranged.Msg.CompletedCount)
	require.Equal(t, int32(1), ranged.Msg.DraftCount)
	require.Equal(t, int32(0), ranged.Msg.VarianceLines)
	require.Equal(t, int64(0), ranged.Msg.LastCompletedAt)
}

func TestGetStocktakeSummary_ScopedToActiveWarehouse(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	startDraft(t, svc, ctx, "in main")

	// A caller whose active warehouse is a different one sees none of it.
	otherWH := seedWarehouse(t, db, "SEC")
	otherCtx := servicetest.CtxInWarehouse(context.Background(), "OWNER", ownerID, otherWH)

	resp, err := svc.GetStocktakeSummary(otherCtx, connect.NewRequest(&stocktakeifacev1.GetStocktakeSummaryRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(0), resp.Msg.DraftCount)
}

func TestGetStocktakeSummary_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB, _ := servicetest.New(t)
	svc := stocktakesvc.NewStocktakeService(gormDB)

	_, err := svc.GetStocktakeSummary(context.Background(),
		connect.NewRequest(&stocktakeifacev1.GetStocktakeSummaryRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
