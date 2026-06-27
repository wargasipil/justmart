package priceagreement

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// UnarchivePriceAgreement restores a soft-deleted agreement (active=true). It
// re-checks the partial unique index on (supplier, product, unit) WHERE active:
// another active agreement may have taken that slot while this one was archived,
// so restoring would collide — reject with the same token as a duplicate create.
func (s *PriceAgreementService) UnarchivePriceAgreement(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UnarchivePriceAgreementRequest],
) (*connect.Response[inventoryifacev1.UnarchivePriceAgreementResponse], error) {
	pa, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if taken, e := common.ExistsBy(s.db.WithContext(ctx), &model.PriceAgreement{},
		"supplier_id = ? AND product_id = ? AND product_unit_id = ? AND active AND id <> ?",
		pa.SupplierID, pa.ProductID, pa.ProductUnitID, pa.ID); e != nil {
		return nil, connect.NewError(connect.CodeInternal, e)
	} else if taken {
		return nil, common.TokenError(connect.CodeAlreadyExists, "price_agreement.exists")
	}
	if err := s.db.WithContext(ctx).Model(pa).Update("active", true).Error; err != nil {
		// Backstop: a rare race lost the pre-check; never leak the raw constraint.
		return nil, common.TokenError(connect.CodeAlreadyExists, "price_agreement.exists")
	}
	pa.Active = true
	return connect.NewResponse(&inventoryifacev1.UnarchivePriceAgreementResponse{Agreement: agreementToProto(pa)}), nil
}
