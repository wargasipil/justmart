package user

import (
	"context"

	"connectrpc.com/connect"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ResolveUsers returns minimal display refs for a set of ids — the
// batch-resolve-by-IDs pattern (mirrors ResolveCustomers). Used by analytics
// pages and the order-history "Created by" column.
func (s *UserService) ResolveUsers(
	ctx context.Context,
	req *connect.Request[userifacev1.ResolveUsersRequest],
) (*connect.Response[userifacev1.ResolveUsersResponse], error) {
	ids := common.DedupeIDs(req.Msg.Ids)
	if len(ids) == 0 {
		return connect.NewResponse(&userifacev1.ResolveUsersResponse{}), nil
	}
	type row struct {
		ID    string `gorm:"column:id"`
		Name  string `gorm:"column:name"`
		Email string `gorm:"column:email"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Model(&model.User{}).
		Select("id, name, email").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*userifacev1.UserRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &userifacev1.UserRef{Id: r.ID, Name: r.Name, Email: r.Email})
	}
	return connect.NewResponse(&userifacev1.ResolveUsersResponse{Users: out}), nil
}
