package user

import (
	"context"

	"connectrpc.com/connect"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
)

func (s *UserService) SetUserActive(
	ctx context.Context,
	req *connect.Request[userifacev1.SetUserActiveRequest],
) (*connect.Response[userifacev1.SetUserActiveResponse], error) {
	user, err := s.loadByID(ctx, req.Msg.UserId)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(user).Update("active", req.Msg.Active).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	user.Active = req.Msg.Active
	return connect.NewResponse(&userifacev1.SetUserActiveResponse{User: UserToProto(user)}), nil
}
