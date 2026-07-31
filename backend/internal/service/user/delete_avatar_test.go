package user_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
	usersvc "github.com/justmart/backend/internal/service/user"
)

func TestDeleteAvatar_RemovesBothRenditionsAndMarker(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.UploadAvatar(ctx, connect.NewRequest(&userifacev1.UploadAvatarRequest{
		ImageData: fakeImage(512), ThumbData: fakeThumb(64), ContentType: "image/jpeg",
	}))
	require.NoError(t, err)

	_, err = svc.DeleteAvatar(ctx, connect.NewRequest(&userifacev1.DeleteAvatarRequest{}))
	require.NoError(t, err)

	// Both renditions gone.
	thumb, err := svc.GetAvatar(ctx, connect.NewRequest(&userifacev1.GetAvatarRequest{}))
	require.NoError(t, err)
	require.Empty(t, thumb.Msg.ImageData)
	orig, err := svc.GetAvatar(ctx, connect.NewRequest(&userifacev1.GetAvatarRequest{
		Variant: userifacev1.AvatarVariant_AVATAR_VARIANT_ORIGINAL,
	}))
	require.NoError(t, err)
	require.Empty(t, orig.Msg.ImageData)

	// And the denormalized marker is cleared in the same tx, so `users` can
	// never advertise an avatar the bytes table no longer has.
	list, err := svc.ListUsers(ctx, connect.NewRequest(&userifacev1.ListUsersRequest{}))
	require.NoError(t, err)
	for _, u := range list.Msg.Users {
		if u.Id == ownerID {
			require.Zero(t, u.AvatarUpdatedAt)
		}
	}
}

func TestDeleteAvatar_Idempotent(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// Deleting when there is nothing to delete succeeds.
	_, err := svc.DeleteAvatar(ctx, connect.NewRequest(&userifacev1.DeleteAvatarRequest{}))
	require.NoError(t, err)
	_, err = svc.DeleteAvatar(ctx, connect.NewRequest(&userifacev1.DeleteAvatarRequest{}))
	require.NoError(t, err)
}

func TestDeleteAvatar_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.DeleteAvatar(context.Background(), connect.NewRequest(&userifacev1.DeleteAvatarRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
