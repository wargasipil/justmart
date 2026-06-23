package customer

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	customerifacev1 "github.com/justmart/backend/gen/customer_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

func (s *CustomerService) UpdateCustomer(
	ctx context.Context,
	req *connect.Request[customerifacev1.UpdateCustomerRequest],
) (*connect.Response[customerifacev1.UpdateCustomerResponse], error) {
	cust, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "customer.name_required")
	}
	updates := map[string]any{
		"name":    name,
		"phone":   strings.TrimSpace(req.Msg.Phone),
		"address": strings.TrimSpace(req.Msg.Address),
		"notes":   strings.TrimSpace(req.Msg.Notes),
	}
	if err := s.db.WithContext(ctx).Model(cust).Updates(updates).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	cust, err = s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&customerifacev1.UpdateCustomerResponse{Customer: customerToProto(cust)}), nil
}
