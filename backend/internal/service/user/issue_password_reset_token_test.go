package user_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	authifacev1 "github.com/justmart/backend/gen/auth_iface/v1"
	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	usersvc "github.com/justmart/backend/internal/service/user"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestIssuePasswordResetToken_Success(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	target, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "resettarget@test.local",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)

	resp, err := svc.IssuePasswordResetToken(ctx, connect.NewRequest(&userifacev1.IssuePasswordResetTokenRequest{
		UserId: target.Msg.User.Id,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Token)                                   // raw token returned once
	require.Greater(t, resp.Msg.ExpiresAt, time.Now().Add(time.Hour).Unix()) // ~24h TTL
}

func TestIssuePasswordResetToken_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)

	// No principal -> MustPrincipal returns Unauthenticated.
	_, err := svc.IssuePasswordResetToken(context.Background(), connect.NewRequest(&userifacev1.IssuePasswordResetTokenRequest{
		UserId: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestIssuePasswordResetToken_EmptyUserID(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.IssuePasswordResetToken(ctx, connect.NewRequest(&userifacev1.IssuePasswordResetTokenRequest{
		UserId: "", // required -> InvalidArgument
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestIssuePasswordResetToken_UserNotFound(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.IssuePasswordResetToken(ctx, connect.NewRequest(&userifacev1.IssuePasswordResetTokenRequest{
		UserId: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
