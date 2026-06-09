package customer

import (
	"context"

	"connectrpc.com/connect"

	customerifacev1 "github.com/justmart/backend/gen/customer_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ResolveCustomers returns minimal display refs for a set of ids. Unknown ids
// are omitted; empty input returns an empty list. No enrich, no preload.
func (s *CustomerService) ResolveCustomers(
	ctx context.Context,
	req *connect.Request[customerifacev1.ResolveCustomersRequest],
) (*connect.Response[customerifacev1.ResolveCustomersResponse], error) {
	ids := common.DedupeIDs(req.Msg.Ids)
	if len(ids) == 0 {
		return connect.NewResponse(&customerifacev1.ResolveCustomersResponse{}), nil
	}
	type row struct {
		ID   string `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Model(&model.Customer{}).
		Select("id, name").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*customerifacev1.CustomerRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &customerifacev1.CustomerRef{Id: r.ID, Name: r.Name})
	}
	return connect.NewResponse(&customerifacev1.ResolveCustomersResponse{Customers: out}), nil
}
