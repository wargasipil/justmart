package priceagreement

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

// ArchivePriceAgreement soft-deletes an agreement (active=false), which also
// frees the (supplier, product, unit) slot for a new active agreement.
func (s *PriceAgreementService) ArchivePriceAgreement(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ArchivePriceAgreementRequest],
) (*connect.Response[inventoryifacev1.ArchivePriceAgreementResponse], error) {
	pa, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(pa).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pa.Active = false
	return connect.NewResponse(&inventoryifacev1.ArchivePriceAgreementResponse{Agreement: agreementToProto(pa)}), nil
}
