package auth_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	coreauth "github.com/justmart/backend/internal/auth"
	authsvc "github.com/justmart/backend/internal/service/auth"
	"github.com/justmart/backend/internal/service/servicetest"
)

// newAuthSvcWithOwner returns the AuthService plus the seeded owner's user id,
// so Me can be called with a principal that resolves to a real users row.
func newAuthSvcWithOwner(t *testing.T) (*authsvc.AuthService, string) {
	t.Helper()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	issuer := &coreauth.Issuer{Secret: []byte(cfg.Auth.JWTSecret), TTL: cfg.Auth.AccessTokenTTL}
	refresh := &coreauth.RefreshIssuer{DB: gormDB, TTL: cfg.Auth.RefreshTokenTTL}
	limiter := coreauth.NewLoginLimiter(1000, 0)
	return authsvc.NewAuthService(gormDB, issuer, refresh, limiter), ownerID
}

func TestMe_Success(t *testing.T) {
	t.Parallel()
	svc, ownerID := newAuthSvcWithOwner(t)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	resp, err := svc.Me(ctx, connect.NewRequest(&userifacev1.MeRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.User)
	require.Equal(t, ownerID, resp.Msg.User.Id)
	require.Equal(t, servicetest.OwnerEmail, resp.Msg.User.Email)
	require.True(t, resp.Msg.User.Active)
}

func TestMe_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc, _ := newAuthSvcWithOwner(t)

	// No principal in the context -> MustPrincipal returns Unauthenticated.
	_, err := svc.Me(context.Background(), connect.NewRequest(&userifacev1.MeRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
