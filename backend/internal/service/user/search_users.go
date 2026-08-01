package user

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// SearchUsers — server-side fuzzy search for the pickers that need to name a
// person: the warehouse-detail "Add user" grant, the resep "doctor / penerbit"
// field, and the cashier-scope filter on order history + analytics. ILIKE on
// email + name.
//
// Two things are narrowed here rather than at each call site:
//
//   - **Only ACTIVE users.** A deactivated account is not a valid pick for any
//     caller — you can't grant it warehouse access, it can't author a script,
//     and offering it as a filter option resurrects ex-staff in every dropdown.
//   - **`with_sales_only`** restricts to people with at least one COMPLETED
//     sale, so the cashier filter can't offer an option that yields an empty
//     table. Opt-in, because the grant/authoring pickers need the full roster.
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
		ID              string     `gorm:"column:id"`
		Name            string     `gorm:"column:name"`
		Email           string     `gorm:"column:email"`
		Role            string     `gorm:"column:role"`
		AvatarUpdatedAt *time.Time `gorm:"column:avatar_updated_at"`
	}
	tx := s.db.WithContext(ctx).
		Model(&model.User{}).
		Select("id, name, email, role, avatar_updated_at").
		Where("active")
	if q != "" {
		like := "%" + q + "%"
		tx = tx.Where("email "+common.LikeOp(tx)+" ? OR name "+common.LikeOp(tx)+" ?", like, like)
	}
	if req.Msg.WithSalesOnly {
		// Correlated EXISTS rather than a JOIN — no DISTINCT needed, and it reads
		// the same on both engines.
		sub := s.db.WithContext(ctx).
			Model(&model.Sale{}).
			Select("1").
			Where("sales.cashier_user_id = users.id AND sales.status = ?", common.SaleStatusCompleted)
		tx = tx.Where("EXISTS (?)", sub)
	}
	var rows []row
	if err := tx.Order("email ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*userifacev1.UserRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &userifacev1.UserRef{
			Id:              r.ID,
			Name:            r.Name,
			Email:           r.Email,
			Role:            roleToProto(r.Role),
			AvatarUpdatedAt: avatarUnix(r.AvatarUpdatedAt),
		})
	}
	return connect.NewResponse(&userifacev1.SearchUsersResponse{Users: out}), nil
}
