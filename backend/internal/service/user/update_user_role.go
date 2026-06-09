package user

import (
	"context"

	"connectrpc.com/connect"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
)

func (s *UserService) UpdateUserRole(
	ctx context.Context,
	req *connect.Request[userifacev1.UpdateUserRoleRequest],
) (*connect.Response[userifacev1.UpdateUserRoleResponse], error) {
	roleStr, err := roleFromProto(req.Msg.Role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	user, err := s.loadByID(ctx, req.Msg.UserId)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(user).Update("role", roleStr).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	user.Role = roleStr
	return connect.NewResponse(&userifacev1.UpdateUserRoleResponse{User: UserToProto(user)}), nil
}
