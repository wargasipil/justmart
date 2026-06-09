package user

import (
	"context"

	"connectrpc.com/connect"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *UserService) ListUsers(
	ctx context.Context,
	_ *connect.Request[userifacev1.ListUsersRequest],
) (*connect.Response[userifacev1.ListUsersResponse], error) {
	var rows []model.User
	if err := s.db.WithContext(ctx).Order("created_at").Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*userifacev1.User, 0, len(rows))
	for _, r := range rows {
		out = append(out, UserToProto(&r))
	}
	return connect.NewResponse(&userifacev1.ListUsersResponse{Users: out}), nil
}
