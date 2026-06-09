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

func TestStartStocktake_DefaultWarehouse(t *testing.T) {
	t.Parallel()
	svc, _, ctx, _ := newSvc(t)

	resp, err := svc.StartStocktake(ctx, connect.NewRequest(&stocktakeifacev1.StartStocktakeRequest{
		Name: "Evening count",
	}))
	require.NoError(t, err)
	sess := resp.Msg.Session
	require.NotNil(t, sess)
	require.NotEmpty(t, sess.Id)
	require.Equal(t, "Evening count", sess.Name)
	require.Equal(t, "DRAFT", sess.Status)
	require.NotEmpty(t, sess.WarehouseId)        // resolved to MAIN
	require.Equal(t, int32(0), sess.LineCount)   // no lines yet
}

func TestStartStocktake_RejectsSecondDraftSameWarehouse(t *testing.T) {
	t.Parallel()
	svc, _, ctx, _ := newSvc(t)

	startDraft(t, svc, ctx, "first")

	_, err := svc.StartStocktake(ctx, connect.NewRequest(&stocktakeifacev1.StartStocktakeRequest{
		Name: "second",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestStartStocktake_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB, _ := servicetest.New(t)
	svc := stocktakesvc.NewStocktakeService(gormDB)

	_, err := svc.StartStocktake(context.Background(), connect.NewRequest(&stocktakeifacev1.StartStocktakeRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
