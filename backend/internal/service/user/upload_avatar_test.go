package user_test

import (
	"bytes"
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
	usersvc "github.com/justmart/backend/internal/service/user"
)

// Renditions are opaque bytes to the handler (it never decodes them), so the
// fixtures only need to be distinguishable and correctly sized.
func fakeImage(n int) []byte { return bytes.Repeat([]byte{0xAA}, n) }
func fakeThumb(n int) []byte { return bytes.Repeat([]byte{0xBB}, n) }

func TestUploadAvatar_StoresBothRenditions(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	res, err := svc.UploadAvatar(ctx, connect.NewRequest(&userifacev1.UploadAvatarRequest{
		ImageData:   fakeImage(4096),
		ThumbData:   fakeThumb(256),
		ContentType: "image/jpeg",
	}))
	require.NoError(t, err)
	require.NotZero(t, res.Msg.AvatarUpdatedAt)

	// Both renditions must come back distinctly — a handler that stored the
	// original in both columns would pass a single-variant test.
	thumb, err := svc.GetAvatar(ctx, connect.NewRequest(&userifacev1.GetAvatarRequest{
		Variant: userifacev1.AvatarVariant_AVATAR_VARIANT_THUMB,
	}))
	require.NoError(t, err)
	require.Equal(t, fakeThumb(256), thumb.Msg.ImageData)

	orig, err := svc.GetAvatar(ctx, connect.NewRequest(&userifacev1.GetAvatarRequest{
		Variant: userifacev1.AvatarVariant_AVATAR_VARIANT_ORIGINAL,
	}))
	require.NoError(t, err)
	require.Equal(t, fakeImage(4096), orig.Msg.ImageData)
}

func TestUploadAvatar_ReplacesExisting(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.UploadAvatar(ctx, connect.NewRequest(&userifacev1.UploadAvatarRequest{
		ImageData: fakeImage(512), ThumbData: fakeThumb(64), ContentType: "image/png",
	}))
	require.NoError(t, err)

	// Re-upload must upsert, not collide on the primary key.
	_, err = svc.UploadAvatar(ctx, connect.NewRequest(&userifacev1.UploadAvatarRequest{
		ImageData: fakeImage(1024), ThumbData: fakeThumb(128), ContentType: "image/webp",
	}))
	require.NoError(t, err)

	got, err := svc.GetAvatar(ctx, connect.NewRequest(&userifacev1.GetAvatarRequest{}))
	require.NoError(t, err)
	require.Equal(t, fakeThumb(128), got.Msg.ImageData)
	require.Equal(t, "image/webp", got.Msg.ContentType)
}

func TestUploadAvatar_ThumbRequired(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	// The two-rendition rule is enforced server-side: an original-only upload
	// is rejected rather than silently leaving list surfaces on heavy bytes.
	_, err := svc.UploadAvatar(ctx, connect.NewRequest(&userifacev1.UploadAvatarRequest{
		ImageData: fakeImage(512), ContentType: "image/jpeg",
	}))
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, connect.CodeInvalidArgument, ce.Code())
	require.Equal(t, "avatar.thumb_required", ce.Message())
}

func TestUploadAvatar_RejectsOversizeAndBadType(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	cases := []struct {
		name  string
		req   *userifacev1.UploadAvatarRequest
		token string
	}{
		{
			name:  "image too large",
			req:   &userifacev1.UploadAvatarRequest{ImageData: fakeImage(usersvc.MaxAvatarBytes + 1), ThumbData: fakeThumb(64), ContentType: "image/jpeg"},
			token: "avatar.image_too_large",
		},
		{
			name:  "thumb too large",
			req:   &userifacev1.UploadAvatarRequest{ImageData: fakeImage(512), ThumbData: fakeThumb(usersvc.MaxAvatarThumbBytes + 1), ContentType: "image/jpeg"},
			token: "avatar.thumb_too_large",
		},
		{
			name:  "svg rejected",
			req:   &userifacev1.UploadAvatarRequest{ImageData: fakeImage(512), ThumbData: fakeThumb(64), ContentType: "image/svg+xml"},
			token: "avatar.content_type_invalid",
		},
		{
			name:  "empty image",
			req:   &userifacev1.UploadAvatarRequest{ThumbData: fakeThumb(64), ContentType: "image/jpeg"},
			token: "avatar.image_required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UploadAvatar(ctx, connect.NewRequest(tc.req))
			require.Error(t, err)
			var ce *connect.Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, connect.CodeInvalidArgument, ce.Code())
			require.Equal(t, tc.token, ce.Message())
		})
	}
}

func TestUploadAvatar_MarksUserRow(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.UploadAvatar(ctx, connect.NewRequest(&userifacev1.UploadAvatarRequest{
		ImageData: fakeImage(512), ThumbData: fakeThumb(64), ContentType: "image/jpeg",
	}))
	require.NoError(t, err)

	// The denormalized marker is what the UI reads to decide whether to fetch.
	list, err := svc.ListUsers(ctx, connect.NewRequest(&userifacev1.ListUsersRequest{}))
	require.NoError(t, err)
	var found bool
	for _, u := range list.Msg.Users {
		if u.Id == ownerID {
			found = true
			require.NotZero(t, u.AvatarUpdatedAt)
		}
	}
	require.True(t, found, "owner must appear in ListUsers")
}

func TestUploadAvatar_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UploadAvatar(context.Background(), connect.NewRequest(&userifacev1.UploadAvatarRequest{
		ImageData: fakeImage(512), ThumbData: fakeThumb(64), ContentType: "image/jpeg",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
