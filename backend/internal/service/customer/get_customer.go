package customer

import (
	"context"

	"connectrpc.com/connect"

	customerifacev1 "github.com/justmart/backend/gen/customer_iface/v1"
)

func (s *CustomerService) GetCustomer(
	ctx context.Context,
	req *connect.Request[customerifacev1.GetCustomerRequest],
) (*connect.Response[customerifacev1.GetCustomerResponse], error) {
	cust, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&customerifacev1.GetCustomerResponse{Customer: customerToProto(cust)}), nil
}
