package customer

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	customerifacev1 "github.com/justmart/backend/gen/customer_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *CustomerService) SearchCustomers(
	ctx context.Context,
	req *connect.Request[customerifacev1.SearchCustomersRequest],
) (*connect.Response[customerifacev1.SearchCustomersResponse], error) {
	query := strings.TrimSpace(req.Msg.Query)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	q := s.db.WithContext(ctx).Where("active = ?", true).Order("name").Limit(limit)
	if query != "" {
		pattern := "%" + query + "%"
		q = q.Where("name "+common.LikeOp(q)+" ? OR phone "+common.LikeOp(q)+" ?", pattern, pattern)
	}

	var rows []model.Customer
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*customerifacev1.Customer, 0, len(rows))
	for _, r := range rows {
		out = append(out, customerToProto(&r))
	}
	return connect.NewResponse(&customerifacev1.SearchCustomersResponse{Customers: out}), nil
}
