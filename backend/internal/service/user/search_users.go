package user

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// SearchUsers — server-side fuzzy search for the warehouse-detail "Add user"
// picker. Mirrors SearchCustomers / SearchSuppliers. ILIKE on email + name.
func (s *UserService) SearchUsers(
	ctx context.Context,
	req *connect.Request[userifacev1.SearchUsersRequest],
) (*connect.Response[userifacev1.SearchUsersResponse], error) {
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := strings.TrimSpace(req.Msg.Query)
	type row struct {
		ID    string `gorm:"column:id"`
		Name  string `gorm:"column:name"`
		Email string `gorm:"column:email"`
	}
	tx := s.db.WithContext(ctx).Model(&model.User{}).Select("id, name, email")
	if q != "" {
		like := "%" + q + "%"
		tx = tx.Where("email "+common.LikeOp(tx)+" ? OR name "+common.LikeOp(tx)+" ?", like, like)
	}
	var rows []row
	if err := tx.Order("email ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*userifacev1.UserRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &userifacev1.UserRef{Id: r.ID, Name: r.Name, Email: r.Email})
	}
	return connect.NewResponse(&userifacev1.SearchUsersResponse{Users: out}), nil
}
