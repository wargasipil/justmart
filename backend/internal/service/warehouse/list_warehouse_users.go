package warehouse

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
)

// ListWarehouseUsers returns the users with access to a warehouse. OWNER only.
func (s *WarehouseService) ListWarehouseUsers(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.ListWarehouseUsersRequest],
) (*connect.Response[warehouseifacev1.ListWarehouseUsersResponse], error) {
	if req.Msg.WarehouseId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("warehouse_id required"))
	}
	type row struct {
		UserID     string `gorm:"column:user_id"`
		Email      string
		Name       string
		Role       string
		IsDefault  bool `gorm:"column:is_default"`
		UserActive bool `gorm:"column:user_active"`
	}
	var rows []row
	err := s.db.WithContext(ctx).Raw(`
		SELECT u.id AS user_id, u.email, u.name, u.role,
		       uw.is_default, u.active AS user_active
		FROM user_warehouses uw
		JOIN users u ON u.id = uw.user_id
		WHERE uw.warehouse_id = ?
		ORDER BY u.email ASC
	`, req.Msg.WarehouseId).Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*warehouseifacev1.WarehouseUser, 0, len(rows))
	for _, r := range rows {
		out = append(out, &warehouseifacev1.WarehouseUser{
			UserId:     r.UserID,
			Email:      r.Email,
			Name:       r.Name,
			Role:       r.Role,
			IsDefault:  r.IsDefault,
			UserActive: r.UserActive,
		})
	}
	return connect.NewResponse(&warehouseifacev1.ListWarehouseUsersResponse{Users: out}), nil
}
