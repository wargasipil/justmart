package user_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	authifacev1 "github.com/justmart/backend/gen/auth_iface/v1"
	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
	usersvc "github.com/justmart/backend/internal/service/user"
)

func TestGetAvatar_DefaultsToThumb(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.UploadAvatar(ctx, connect.NewRequest(&userifacev1.UploadAvatarRequest{
		ImageData: fakeImage(2048), ThumbData: fakeThumb(128), ContentType: "image/jpeg",
	}))
	require.NoError(t, err)

	// Variant unset must resolve to THUMB — the fast path is the default, so a
	// caller can never accidentally pull the original.
	got, err := svc.GetAvatar(ctx, connect.NewRequest(&userifacev1.GetAvatarRequest{}))
	require.NoError(t, err)
	require.Equal(t, userifacev1.AvatarVariant_AVATAR_VARIANT_THUMB, got.Msg.Variant)
	require.Equal(t, fakeThumb(128), got.Msg.ImageData)
	require.Equal(t, "image/jpeg", got.Msg.ContentType)
	require.NotZero(t, got.Msg.AvatarUpdatedAt)
}

func TestGetAvatar_NoAvatarIsNotAnError(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Absence is an empty response, not NotFound — the UI renders initials.
	got, err := svc.GetAvatar(ctx, connect.NewRequest(&userifacev1.GetAvatarRequest{}))
	require.NoError(t, err)
	require.Empty(t, got.Msg.ImageData)
	require.Zero(t, got.Msg.AvatarUpdatedAt)
}

func TestGetAvatar_OtherUserReadable(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ownerCtx := servicetest.OwnerCtx(context.Background(), ownerID)

	cashier, err := svc.CreateUser(ownerCtx, connect.NewRequest(&userifacev1.CreateUserRequest{
		Email: "avatar-viewer@test.local", Password: "supersecret",
		Role: authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)

	_, err = svc.UploadAvatar(ownerCtx, connect.NewRequest(&userifacev1.UploadAvatarRequest{
		ImageData: fakeImage(512), ThumbData: fakeThumb(64), ContentType: "image/png",
	}))
	require.NoError(t, err)

	// A cashier may read a colleague's avatar (user lists, "Created by" column).
	cashierCtx := servicetest.CtxAs(context.Background(), "CASHIER", cashier.Msg.User.Id)
	got, err := svc.GetAvatar(cashierCtx, connect.NewRequest(&userifacev1.GetAvatarRequest{
		UserId: ownerID,
	}))
	require.NoError(t, err)
	require.Equal(t, fakeThumb(64), got.Msg.ImageData)
}

func TestGetAvatar_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.GetAvatar(context.Background(), connect.NewRequest(&userifacev1.GetAvatarRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
