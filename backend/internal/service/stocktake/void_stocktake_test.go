package stocktake_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
)

func TestVoidStocktake_VoidsDraft(t *testing.T) {
	t.Parallel()
	svc, _, ctx, _ := newSvc(t)
	sessionID := startDraft(t, svc, ctx, "to void")

	resp, err := svc.VoidStocktake(ctx, connect.NewRequest(&stocktakeifacev1.VoidStocktakeRequest{
		SessionId: sessionID,
	}))
	require.NoError(t, err)
	require.Equal(t, "VOIDED", resp.Msg.Session.Status)
	require.NotZero(t, resp.Msg.Session.VoidedAt)
}

func TestVoidStocktake_AlreadyVoided(t *testing.T) {
	t.Parallel()
	svc, _, ctx, _ := newSvc(t)
	sessionID := startDraft(t, svc, ctx, "to void")

	_, err := svc.VoidStocktake(ctx, connect.NewRequest(&stocktakeifacev1.VoidStocktakeRequest{SessionId: sessionID}))
	require.NoError(t, err)

	// Voiding a non-DRAFT session fails the lockDraftSession precondition.
	_, err = svc.VoidStocktake(ctx, connect.NewRequest(&stocktakeifacev1.VoidStocktakeRequest{SessionId: sessionID}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestVoidStocktake_NotFound(t *testing.T) {
	t.Parallel()
	svc, _, ctx, _ := newSvc(t)

	_, err := svc.VoidStocktake(ctx, connect.NewRequest(&stocktakeifacev1.VoidStocktakeRequest{
		SessionId: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
