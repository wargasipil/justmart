package customer

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	customerifacev1 "github.com/justmart/backend/gen/customer_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *CustomerService) ListCustomers(
	ctx context.Context,
	req *connect.Request[customerifacev1.ListCustomersRequest],
) (*connect.Response[customerifacev1.ListCustomersResponse], error) {
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	query := strings.TrimSpace(req.Msg.Query)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		if !req.Msg.IncludeInactive {
			q = q.Where("active = ?", true)
		}
		if query != "" {
			pattern := "%" + query + "%"
			q = q.Where("name "+common.LikeOp(q)+" ? OR phone "+common.LikeOp(q)+" ?", pattern, pattern)
		}
		return q
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Customer{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.Customer
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Customer{})).
		Order("name").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*customerifacev1.Customer, 0, len(rows))
	for _, r := range rows {
		out = append(out, customerToProto(&r))
	}
	return connect.NewResponse(&customerifacev1.ListCustomersResponse{
		Customers: out,
		Total:     int32(total),
	}), nil
}
