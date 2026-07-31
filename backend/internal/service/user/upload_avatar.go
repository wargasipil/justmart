package user

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// UploadAvatar replaces the caller's own profile picture. Self-only by
// construction: the request carries no user_id, so there is no path to write
// someone else's avatar and nothing to guard.
//
// Both renditions are required — a row with only an original would silently
// push every avatar/list surface onto the heavy bytes, which is exactly what
// the two-rendition rule exists to prevent.
func (s *UserService) UploadAvatar(
	ctx context.Context,
	req *connect.Request[userifacev1.UploadAvatarRequest],
) (*connect.Response[userifacev1.UploadAvatarResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	image := req.Msg.ImageData
	thumb := req.Msg.ThumbData
	if len(image) == 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "avatar.image_required")
	}
	if len(thumb) == 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "avatar.thumb_required")
	}
	if len(image) > MaxAvatarBytes {
		return nil, common.TokenError(connect.CodeInvalidArgument, "avatar.image_too_large")
	}
	if len(thumb) > MaxAvatarThumbBytes {
		return nil, common.TokenError(connect.CodeInvalidArgument, "avatar.thumb_too_large")
	}
	if !allowedAvatarTypes[req.Msg.ContentType] {
		return nil, common.TokenError(connect.CodeInvalidArgument, "avatar.content_type_invalid")
	}

	now := time.Now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := model.UserAvatar{
			UserID:      caller.UserID,
			ContentType: req.Msg.ContentType,
			ImageData:   image,
			ThumbData:   thumb,
			UpdatedAt:   now,
		}
		// Upsert: re-uploading replaces both renditions rather than erroring on
		// the primary key.
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"content_type", "image_data", "thumb_data", "updated_at",
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
		// Keep the denormalized marker on `users` in step, in the same tx — it
		// is what every other read uses to decide whether an avatar exists.
		return tx.Model(&model.User{}).
			Where("id = ?", caller.UserID).
			Update("avatar_updated_at", now).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	return connect.NewResponse(&userifacev1.UploadAvatarResponse{
		AvatarUpdatedAt: now.Unix(),
	}), nil
}
