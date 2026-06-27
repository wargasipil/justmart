package priceagreement

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// CreatePriceAgreements creates many product/unit lines for ONE supplier in a
// single transaction (all-or-nothing) — any invalid line rolls back the whole
// batch, so zero rows are created. Backs the dedicated "New price agreement"
// page; shares per-line validation with the single CreatePriceAgreement via
// buildAgreement so tokens stay identical.
func (s *PriceAgreementService) CreatePriceAgreements(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreatePriceAgreementsRequest],
) (*connect.Response[inventoryifacev1.CreatePriceAgreementsResponse], error) {
	supplierID := strings.TrimSpace(req.Msg.SupplierId)
	if supplierID == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "price_agreement.required")
	}
	if len(req.Msg.Items) == 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "price_agreement.items_required")
	}

	db := s.db.WithContext(ctx)
	if ok, e := common.ExistsBy(db, &model.Supplier{}, "id = ?", supplierID); e != nil {
		return nil, connect.NewError(connect.CodeInternal, e)
	} else if !ok {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "price_agreement.supplier_missing")
	}

	created := make([]*model.PriceAgreement, 0, len(req.Msg.Items))
	err := db.Transaction(func(tx *gorm.DB) error {
		seen := make(map[string]bool, len(req.Msg.Items)) // within-request dedupe
		for _, it := range req.Msg.Items {
			key := strings.TrimSpace(it.ProductId) + "|" + strings.TrimSpace(it.ProductUnitId)
			if seen[key] {
				return common.TokenError(connect.CodeAlreadyExists, "price_agreement.exists")
			}
			seen[key] = true

			pa, err := buildAgreement(ctx, tx, supplierID,
				it.ProductId, it.ProductUnitId, it.Price,
				it.ValidFrom, it.ValidUntil, it.Note)
			if err != nil {
				return err
			}
			if err := tx.Create(pa).Error; err != nil {
				// Unique-index race backstop.
				return common.TokenError(connect.CodeAlreadyExists, "price_agreement.exists")
			}
			created = append(created, pa)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := make([]*inventoryifacev1.PriceAgreement, 0, len(created))
	for _, pa := range created {
		out = append(out, agreementToProto(pa))
	}
	return connect.NewResponse(&inventoryifacev1.CreatePriceAgreementsResponse{Agreements: out}), nil
}
