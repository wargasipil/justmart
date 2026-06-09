package customer

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	customerifacev1 "github.com/justmart/backend/gen/customer_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// load fetches a single customer by id, mapping not-found / empty-id to the
// appropriate Connect error codes.
func (s *CustomerService) load(ctx context.Context, id string) (*model.Customer, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var row model.Customer
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("customer %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &row, nil
}

func customerToProto(c *model.Customer) *customerifacev1.Customer {
	return &customerifacev1.Customer{
		Id:        c.ID,
		Name:      c.Name,
		Phone:     c.Phone,
		Address:   c.Address,
		Notes:     c.Notes,
		Active:    c.Active,
		CreatedAt: c.CreatedAt.Unix(),
	}
}
