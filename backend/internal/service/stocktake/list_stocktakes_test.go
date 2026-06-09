package stocktake_test

import (
	"context"
	"testing"

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

func TestListStocktakes_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB, _ := servicetest.New(t)
	svc := stocktakesvc.NewStocktakeService(gormDB)

	_, err := svc.ListStocktakes(context.Background(), connect.NewRequest(&stocktakeifacev1.ListStocktakesRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
