package supplier

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *SupplierService) CreateSupplier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreateSupplierRequest],
) (*connect.Response[inventoryifacev1.CreateSupplierResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	code := strings.ToUpper(strings.TrimSpace(req.Msg.Code))
	if name == "" || code == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("code and name required"))
	}
	sup := model.Supplier{
		Code:         code,
		Name:         name,
		ContactEmail: strings.TrimSpace(req.Msg.ContactEmail),
		Phone:        strings.TrimSpace(req.Msg.Phone),
		Active:       true,
	}
	if err := s.db.WithContext(ctx).Create(&sup).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("create supplier: %w", err))
	}
	return connect.NewResponse(&inventoryifacev1.CreateSupplierResponse{Supplier: supplierToProto(&sup)}), nil
}
