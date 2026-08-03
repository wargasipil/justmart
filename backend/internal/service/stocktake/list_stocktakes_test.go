package stocktake_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	stocktakesvc "github.com/justmart/backend/internal/service/stocktake"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestListStocktakes_ReturnsActiveWarehouseSessions(t *testing.T) {
	t.Parallel()
	svc, _, ctx, _ := newSvc(t)
	sessionID := startDraft(t, svc, ctx, "count")

	resp, err := svc.ListStocktakes(ctx, connect.NewRequest(&stocktakeifacev1.ListStocktakesRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Len(t, resp.Msg.Sessions, 1)
	require.Equal(t, sessionID, resp.Msg.Sessions[0].Id)
	require.Equal(t, "DRAFT", resp.Msg.Sessions[0].Status)
}

func TestListStocktakes_StatusFilter(t *testing.T) {
	t.Parallel()
	svc, _, ctx, _ := newSvc(t)
	sessionID := startDraft(t, svc, ctx, "count")
	// Void it so it leaves the DRAFT bucket.
	_, err := svc.VoidStocktake(ctx, connect.NewRequest(&stocktakeifacev1.VoidStocktakeRequest{SessionId: sessionID}))
	require.NoError(t, err)

	// Filtering by DRAFT now returns nothing.
	draftResp, err := svc.ListStocktakes(ctx, connect.NewRequest(&stocktakeifacev1.ListStocktakesRequest{
		Status: "DRAFT",
	}))
	require.NoError(t, err)
	require.Equal(t, int32(0), draftResp.Msg.Total)

	// Filtering by VOIDED returns the one session.
	voidedResp, err := svc.ListStocktakes(ctx, connect.NewRequest(&stocktakeifacev1.ListStocktakesRequest{
		Status: "VOIDED",
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), voidedResp.Msg.Total)
}

func TestListStocktakes_DateRangeOnCreated(t *testing.T) {
	t.Parallel()
	svc, db, ctx, _ := newSvc(t)
	oldID := startDraft(t, svc, ctx, "old")
	// Void it so a second session can be started (one DRAFT per warehouse).
	_, err := svc.VoidStocktake(ctx, connect.NewRequest(&stocktakeifacev1.VoidStocktakeRequest{SessionId: oldID}))
	require.NoError(t, err)
	backdateSession(t, db, oldID, time.Now().AddDate(0, 0, -90), time.Time{})
	startDraft(t, svc, ctx, "recent")

	// No bounds ("Any date") sees both.
	all, err := svc.ListStocktakes(ctx, connect.NewRequest(&stocktakeifacev1.ListStocktakesRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(2), all.Msg.Total)

	// Last 30 days drops the backdated one.
	recent, err := svc.ListStocktakes(ctx, connect.NewRequest(&stocktakeifacev1.ListStocktakesRequest{
		FromUnix: time.Now().AddDate(0, 0, -30).Unix(),
		ToUnix:   time.Now().Add(time.Hour).Unix(),
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), recent.Msg.Total)
	require.Equal(t, "recent", recent.Msg.Sessions[0].Name)
}

func TestListStocktakes_DateRangeOnCompleted(t *testing.T) {
	t.Parallel()
	svc, db, ctx, ownerID := newSvc(t)
	whID := defaultWarehouseID(t, db)

	// A session created long ago but completed today.
	sessionID := startDraft(t, svc, ctx, "late finish")
	batch := seedBatchWithStock(t, db, ownerID, whID, 4)
	recordCount(t, svc, ctx, addBatch(t, svc, db, ctx, sessionID, batch), 4)
	_, err := svc.CompleteStocktake(ctx, connect.NewRequest(&stocktakeifacev1.CompleteStocktakeRequest{
		SessionId: sessionID,
	}))
	require.NoError(t, err)
	backdateSession(t, db, sessionID, time.Now().AddDate(0, 0, -90), time.Now())

	from := time.Now().AddDate(0, 0, -30).Unix()
	to := time.Now().Add(time.Hour).Unix()

	// By created it falls outside the window...
	byCreated, err := svc.ListStocktakes(ctx, connect.NewRequest(&stocktakeifacev1.ListStocktakesRequest{
		FromUnix: from, ToUnix: to, DateField: "created",
	}))
	require.NoError(t, err)
	require.Equal(t, int32(0), byCreated.Msg.Total)

	// ...but by completed it's inside.
	byCompleted, err := svc.ListStocktakes(ctx, connect.NewRequest(&stocktakeifacev1.ListStocktakesRequest{
		FromUnix: from, ToUnix: to, DateField: "completed",
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), byCompleted.Msg.Total)
}

func TestListStocktakes_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB, _ := servicetest.New(t)
	svc := stocktakesvc.NewStocktakeService(gormDB)

	_, err := svc.ListStocktakes(context.Background(), connect.NewRequest(&stocktakeifacev1.ListStocktakesRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
