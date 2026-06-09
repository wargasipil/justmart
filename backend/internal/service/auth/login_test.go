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

// newAuthSvc wires an AuthService against a fresh throwaway SQLite DB with the
// bootstrap owner seeded. The login limiter is intentionally generous so a
// `-count=N` run never trips ResourceExhausted.
func newAuthSvc(t *testing.T) *authsvc.AuthService {
	t.Helper()
	gormDB, cfg := servicetest.New(t)
	servicetest.EnsureOwner(t, gormDB, cfg) // creates owner@test.local
	issuer := &coreauth.Issuer{Secret: []byte(cfg.Auth.JWTSecret), TTL: cfg.Auth.AccessTokenTTL}
	refresh := &coreauth.RefreshIssuer{DB: gormDB, TTL: cfg.Auth.RefreshTokenTTL}
	limiter := coreauth.NewLoginLimiter(1000, 0)
	return authsvc.NewAuthService(gormDB, issuer, refresh, limiter)
}

func TestLogin_Success(t *testing.T) {
	t.Parallel()
	svc := newAuthSvc(t)

	resp, err := svc.Login(context.Background(), connect.NewRequest(&userifacev1.LoginRequest{
		Email:    servicetest.OwnerEmail,
		Password: servicetest.OwnerPassword,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.AccessToken)
	require.NotEmpty(t, resp.Msg.RefreshToken)
	require.NotNil(t, resp.Msg.User)
	require.Equal(t, servicetest.OwnerEmail, resp.Msg.User.Email)
}

func TestLogin_WrongPassword(t *testing.T) {
	t.Parallel()
	svc := newAuthSvc(t)

	_, err := svc.Login(context.Background(), connect.NewRequest(&userifacev1.LoginRequest{
		Email:    servicetest.OwnerEmail,
		Password: "definitely-wrong",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
