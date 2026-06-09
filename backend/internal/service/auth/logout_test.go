package auth_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
)

func TestLogout_Success(t *testing.T) {
	t.Parallel()
	svc := newAuthSvc(t)
	raw := loginRefreshToken(t, svc)

	resp, err := svc.Logout(context.Background(), connect.NewRequest(&userifacev1.LogoutRequest{
		RefreshToken: raw,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg)

	// Logout revokes the whole family, so the token can no longer rotate.
	_, err = svc.Refresh(context.Background(), connect.NewRequest(&userifacev1.RefreshRequest{
		RefreshToken: raw,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

// Logout is best-effort: an unknown or empty token succeeds silently (so a
// double-logout never errors).
func TestLogout_UnknownTokenSucceeds(t *testing.T) {
	t.Parallel()
	svc := newAuthSvc(t)

	_, err := svc.Logout(context.Background(), connect.NewRequest(&userifacev1.LogoutRequest{
		RefreshToken: "never-issued",
	}))
	require.NoError(t, err)

	_, err = svc.Logout(context.Background(), connect.NewRequest(&userifacev1.LogoutRequest{
		RefreshToken: "",
	}))
	require.NoError(t, err)
}
