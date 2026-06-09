package auth

import (
	"context"

	"connectrpc.com/connect"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	coreauth "github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	usersvc "github.com/justmart/backend/internal/service/user"
)

func (s *AuthService) Me(
	ctx context.Context,
	_ *connect.Request[userifacev1.MeRequest],
) (*connect.Response[userifacev1.MeResponse], error) {
	p, err := coreauth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var user model.User
	if err := s.db.WithContext(ctx).Where("id = ?", p.UserID).First(&user).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&userifacev1.MeResponse{User: usersvc.UserToProto(&user)}), nil
}
