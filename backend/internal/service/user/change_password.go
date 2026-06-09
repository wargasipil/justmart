package user

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/auth"
)

func (s *UserService) ChangePassword(
	ctx context.Context,
	req *connect.Request[userifacev1.ChangePasswordRequest],
) (*connect.Response[userifacev1.ChangePasswordResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if len(req.Msg.NewPassword) < 8 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("new_password must be at least 8 characters"))
	}

	targetID := req.Msg.UserId
	isSelf := targetID == "" || targetID == caller.UserID
	if !isSelf && caller.Role != roleOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only OWNER can change another user's password"))
	}
	if isSelf {
		targetID = caller.UserID
	}

	target, err := s.loadByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if isSelf {
		if err := auth.VerifyPassword(target.PasswordHash, req.Msg.OldPassword); err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("old_password incorrect"))
		}
	}

	hash, err := auth.HashPassword(req.Msg.NewPassword)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := s.db.WithContext(ctx).Model(target).Update("password_hash", hash).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&userifacev1.ChangePasswordResponse{}), nil
}
