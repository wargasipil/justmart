package warehouse

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *WarehouseService) RevokeWarehouseAccess(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.RevokeWarehouseAccessRequest],
) (*connect.Response[warehouseifacev1.RevokeWarehouseAccessResponse], error) {
	res := s.db.WithContext(ctx).Where("user_id = ? AND warehouse_id = ?",
		req.Msg.UserId, req.Msg.WarehouseId).Delete(&model.UserWarehouse{})
	if res.Error != nil {
		return nil, connect.NewError(connect.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("membership not found"))
	}
	return connect.NewResponse(&warehouseifacev1.RevokeWarehouseAccessResponse{}), nil
}
