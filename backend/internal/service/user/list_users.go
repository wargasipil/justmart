package user

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *UserService) ListUsers(
	ctx context.Context,
	req *connect.Request[userifacev1.ListUsersRequest],
) (*connect.Response[userifacev1.ListUsersResponse], error) {
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)

	// One filter closure feeds both the count and the page, so the two can't drift.
	applyFilters := func(q *gorm.DB) *gorm.DB { return q.Model(&model.User{}) }

	var total int64
	if err := applyFilters(s.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.User
	// id breaks created_at ties — the bootstrap owner and a seeded user can land
	// on the same timestamp, and an unstable order would shuffle rows across pages.
	if err := applyFilters(s.db.WithContext(ctx)).
		Order("created_at, id").
		Offset(offset).Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*userifacev1.User, 0, len(rows))
	for _, r := range rows {
		out = append(out, UserToProto(&r))
	}
	return connect.NewResponse(&userifacev1.ListUsersResponse{
		Users: out,
		Total: int32(total),
	}), nil
}
