package priceagreement

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *PriceAgreementService) CreatePriceAgreement(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreatePriceAgreementRequest],
) (*connect.Response[inventoryifacev1.CreatePriceAgreementResponse], error) {
	supplierID := strings.TrimSpace(req.Msg.SupplierId)
	if supplierID == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "price_agreement.required")
	}

	db := s.db.WithContext(ctx)
	// Referenced supplier must exist (friendly token, not a raw FK error).
	if ok, e := common.ExistsBy(db, &model.Supplier{}, "id = ?", supplierID); e != nil {
		return nil, connect.NewError(connect.CodeInternal, e)
	} else if !ok {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "price_agreement.supplier_missing")
	}

	pa, err := buildAgreement(ctx, db, supplierID,
		req.Msg.ProductId, req.Msg.ProductUnitId, req.Msg.Price,
		req.Msg.ValidFrom, req.Msg.ValidUntil, req.Msg.Note)
	if err != nil {
		return nil, err
	}
	if err := db.Create(pa).Error; err != nil {
		// Unique-index race backstop.
		return nil, common.TokenError(connect.CodeAlreadyExists, "price_agreement.exists")
	}
	return connect.NewResponse(&inventoryifacev1.CreatePriceAgreementResponse{Agreement: agreementToProto(pa)}), nil
}
