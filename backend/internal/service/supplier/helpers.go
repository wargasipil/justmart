package supplier

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *SupplierService) load(ctx context.Context, id string) (*model.Supplier, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var sup model.Supplier
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&sup).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("supplier %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &sup, nil
}

func supplierToProto(s *model.Supplier) *inventoryifacev1.Supplier {
	return &inventoryifacev1.Supplier{
		Id:           s.ID,
		Code:         s.Code,
		Name:         s.Name,
		ContactEmail: s.ContactEmail,
		Phone:        s.Phone,
		Active:       s.Active,
		CreatedAt:    s.CreatedAt.Unix(),
	}
}
