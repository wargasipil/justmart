package product

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// GetProductExpiryDefaults returns the expiry setting of a set of products —
// what the Receive dialog needs to pre-fill its lines, in one light read
// instead of a full GetProduct per line. Unknown ids are omitted; ids are
// deduped and capped like every Resolve*. No enrich.
//
// prescription_required rides along because the dialog applies a pharmacy rule
// with it (no estimated expiry on a prescription medicine) and would otherwise
// need a second lookup for the one flag.
func (s *ProductService) GetProductExpiryDefaults(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetProductExpiryDefaultsRequest],
) (*connect.Response[inventoryifacev1.GetProductExpiryDefaultsResponse], error) {
	ids := common.DedupeIDs(req.Msg.ProductIds)
	if len(ids) == 0 {
		return connect.NewResponse(&inventoryifacev1.GetProductExpiryDefaultsResponse{}), nil
	}
	type row struct {
		ID                   string `gorm:"column:id"`
		ExpiryDefault        string `gorm:"column:expiry_default"`
		ExpiryDefaultMonths  int32  `gorm:"column:expiry_default_months"`
		PrescriptionRequired bool   `gorm:"column:prescription_required"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Model(&model.Product{}).
		Select("id, expiry_default, expiry_default_months, prescription_required").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.ProductExpiryDefault, 0, len(rows))
	for _, r := range rows {
		out = append(out, &inventoryifacev1.ProductExpiryDefault{
			ProductId:            r.ID,
			ExpiryDefault:        expiryDefaultToProto(r.ExpiryDefault),
			ExpiryDefaultMonths:  r.ExpiryDefaultMonths,
			PrescriptionRequired: r.PrescriptionRequired,
		})
	}
	return connect.NewResponse(&inventoryifacev1.GetProductExpiryDefaultsResponse{Defaults: out}), nil
}
