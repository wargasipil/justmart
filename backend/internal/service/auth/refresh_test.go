package auth_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	authsvc "github.com/justmart/backend/internal/service/auth"
	"github.com/justmart/backend/internal/service/servicetest"
)

// loginRefreshToken logs the bootstrap owner in and returns the freshly minted
// raw refresh token. Refresh/Logout consume that token, so the round-trip starts
// from a real Login rather than hand-crafting a token row.
func loginRefreshToken(t *testing.T, svc *authsvc.AuthService) string {
	t.Helper()
	resp, err := svc.Login(context.Background(), connect.NewRequest(&userifacev1.LoginRequest{
		Email:    servicetest.OwnerEmail,
		Password: servicetest.OwnerPassword,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.RefreshToken)
	return resp.Msg.RefreshToken
}

func TestRefresh_Success(t *testing.T) {
	t.Parallel()
	svc := newAuthSvc(t)
	raw := loginRefreshToken(t, svc)

	resp, err := svc.Refresh(context.Background(), connect.NewRequest(&userifacev1.RefreshRequest{
		RefreshToken: raw,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.AccessToken)
	require.NotEmpty(t, resp.Msg.RefreshToken)
	// Rotation: the returned refresh token must differ from the presented one.
	require.NotEqual(t, raw, resp.Msg.RefreshToken)
	require.Greater(t, resp.Msg.AccessExpiresAt, int64(0))
	require.Greater(t, resp.Msg.RefreshExpiresAt, int64(0))
}

func TestRefresh_ReuseDetected(t *testing.T) {
	t.Parallel()
	svc := newAuthSvc(t)
	raw := loginRefreshToken(t, svc)

	// First rotation consumes (revokes) the presented token.
	_, err := svc.Refresh(context.Background(), connect.NewRequest(&userifacev1.RefreshRequest{
		RefreshToken: raw,
	}))
	require.NoError(t, err)

	// Replaying the now-revoked token is reuse -> Unauthenticated.
	_, err = svc.Refresh(context.Background(), connect.NewRequest(&userifacev1.RefreshRequest{
		RefreshToken: raw,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRefresh_UnknownToken(t *testing.T) {
	t.Parallel()
	svc := newAuthSvc(t)

	_, err := svc.Refresh(context.Background(), connect.NewRequest(&userifacev1.RefreshRequest{
		RefreshToken: "not-a-real-token",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRefresh_EmptyToken(t *testing.T) {
	t.Parallel()
	svc := newAuthSvc(t)

	_, err := svc.Refresh(context.Background(), connect.NewRequest(&userifacev1.RefreshRequest{
		RefreshToken: "",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
