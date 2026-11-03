package manufacturer

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// SearchManufacturers backs <ManufacturerSelect>'s loadOptions: top matches
// only, ACTIVE rows only (you cannot assign an archived pabrik to a product).
func (s *ManufacturerService) SearchManufacturers(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.SearchManufacturersRequest],
) (*connect.Response[inventoryifacev1.SearchManufacturersResponse], error) {
	query := strings.TrimSpace(req.Msg.Query)
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := s.db.WithContext(ctx).Model(&model.Manufacturer{}).Where("active = ?", true)
	if query != "" {
		pattern := "%" + query + "%"
		q = q.Where("name "+common.LikeOp(q)+" ? OR code "+common.LikeOp(q)+" ?", pattern, pattern)
	}
	var rows []model.Manufacturer
	if err := q.Order("name, id").Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Manufacturer, 0, len(rows))
	for i := range rows {
		out = append(out, manufacturerToProto(&rows[i]))
	}
	return connect.NewResponse(&inventoryifacev1.SearchManufacturersResponse{Manufacturers: out}), nil
}
