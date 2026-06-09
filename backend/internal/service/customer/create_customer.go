package customer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"

	customerifacev1 "github.com/justmart/backend/gen/customer_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *CustomerService) CreateCustomer(
	ctx context.Context,
	req *connect.Request[customerifacev1.CreateCustomerRequest],
) (*connect.Response[customerifacev1.CreateCustomerResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name required"))
	}
	row := model.Customer{
		Name:    name,
		Phone:   strings.TrimSpace(req.Msg.Phone),
		Address: strings.TrimSpace(req.Msg.Address),
		Notes:   strings.TrimSpace(req.Msg.Notes),
		Active:  true,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create customer: %w", err))
	}
	return connect.NewResponse(&customerifacev1.CreateCustomerResponse{Customer: customerToProto(&row)}), nil
}
