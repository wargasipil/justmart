package user

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// GetAvatar returns one rendition of a user's profile picture. Any signed-in
// user may read any user's avatar (user lists, the order-history "Created by"
// column) — it exposes nothing a caller can't already see.
//
// A user with no avatar is NOT an error: the response comes back empty with
// avatar_updated_at = 0 so the UI falls through to its initials placeholder
// without special-casing a NotFound.
func (s *UserService) GetAvatar(
	ctx context.Context,
	req *connect.Request[userifacev1.GetAvatarRequest],
) (*connect.Response[userifacev1.GetAvatarResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	targetID := req.Msg.UserId
	if targetID == "" {
		targetID = caller.UserID
	}
	variant := resolveVariant(req.Msg.Variant)

	// Select only the rendition asked for — fetching both would defeat the
	// point of storing a thumbnail.
	column := "thumb_data"
	if variant == userifacev1.AvatarVariant_AVATAR_VARIANT_ORIGINAL {
		column = "image_data"
	}

	var row model.UserAvatar
	err = s.db.WithContext(ctx).
		Select(column+" AS image_data", "content_type", "user_id", "updated_at").
		Where("user_id = ?", targetID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return connect.NewResponse(&userifacev1.GetAvatarResponse{Variant: variant}), nil
	}
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	return connect.NewResponse(&userifacev1.GetAvatarResponse{
		ImageData:       row.ImageData,
		ContentType:     row.ContentType,
		AvatarUpdatedAt: row.UpdatedAt.Unix(),
		Variant:         variant,
	}), nil
}
