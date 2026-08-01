package user

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// DeleteAvatar removes the caller's own profile picture, reverting them to the
// initials placeholder. Self-only by construction (no user_id on the request),
// and idempotent: deleting when there is nothing to delete succeeds.
func (s *UserService) DeleteAvatar(
	ctx context.Context,
	req *connect.Request[userifacev1.DeleteAvatarRequest],
) (*connect.Response[userifacev1.DeleteAvatarResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", caller.UserID).
			Delete(&model.UserAvatar{}).Error; err != nil {
			return err
		}
		// Clear the denormalized marker in the same tx, so `users` can never
		// claim an avatar that the bytes table no longer has.
		return tx.Model(&model.User{}).
			Where("id = ?", caller.UserID).
			Update("avatar_updated_at", nil).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	return connect.NewResponse(&userifacev1.DeleteAvatarResponse{}), nil
}
