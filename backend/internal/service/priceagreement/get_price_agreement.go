package priceagreement

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func (s *PriceAgreementService) GetPriceAgreement(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.GetPriceAgreementRequest],
) (*connect.Response[inventoryifacev1.GetPriceAgreementResponse], error) {
	pa, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.GetPriceAgreementResponse{Agreement: agreementToProto(pa)}), nil
}
