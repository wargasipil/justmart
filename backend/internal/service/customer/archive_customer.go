package customer

import (
	"context"

	"connectrpc.com/connect"

	customerifacev1 "github.com/justmart/backend/gen/customer_iface/v1"
)

func (s *CustomerService) ArchiveCustomer(
	ctx context.Context,
	req *connect.Request[customerifacev1.ArchiveCustomerRequest],
) (*connect.Response[customerifacev1.ArchiveCustomerResponse], error) {
	cust, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(cust).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	cust.Active = false
	return connect.NewResponse(&customerifacev1.ArchiveCustomerResponse{Customer: customerToProto(cust)}), nil
}
