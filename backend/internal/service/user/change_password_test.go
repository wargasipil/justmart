package user_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	authifacev1 "github.com/justmart/backend/gen/auth_iface/v1"
	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	usersvc "github.com/justmart/backend/internal/service/user"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestChangePassword_Self(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg) // owner seeded with OwnerPassword
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Self-change: empty UserId => target is the caller; OldPassword must verify.
	_, err := svc.ChangePassword(ctx, connect.NewRequest(&userifacev1.ChangePasswordRequest{
		UserId:      "",
		OldPassword: servicetest.OwnerPassword,
		NewPassword: "brand-new-password",
	}))
	require.NoError(t, err)
}

func TestChangePassword_WrongOldPassword(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.ChangePassword(ctx, connect.NewRequest(&userifacev1.ChangePasswordRequest{
		OldPassword: "definitely-not-the-password",
		NewPassword: "brand-new-password",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestChangePassword_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	// No principal in ctx -> MustPrincipal returns Unauthenticated.
	_, err := svc.ChangePassword(context.Background(), connect.NewRequest(&userifacev1.ChangePasswordRequest{
		NewPassword: "brand-new-password",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestChangePassword_ShortNewPassword(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.ChangePassword(ctx, connect.NewRequest(&userifacev1.ChangePasswordRequest{
		OldPassword: servicetest.OwnerPassword,
		NewPassword: "short", // < 8 chars -> InvalidArgument
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestChangePassword_OtherUserDeniedForNonOwner(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)

	// Create a cashier (the caller) and a target.
	caller, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "caller-cashier@test.local",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)
	target, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "target-cashier@test.local",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)

	// CASHIER caller trying to change ANOTHER user's password -> PermissionDenied.
	ctx := servicetest.CtxAs(context.Background(), "CASHIER", caller.Msg.User.Id)
	_, err = svc.ChangePassword(ctx, connect.NewRequest(&userifacev1.ChangePasswordRequest{
		UserId:      target.Msg.User.Id,
		NewPassword: "brand-new-password",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}
