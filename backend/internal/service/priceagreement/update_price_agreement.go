package priceagreement

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// UpdatePriceAgreement edits an existing agreement. supplier_id/product_id are
// immutable (re-create for a different pair); the unit, price, dates, and note
// can change. Changing the unit re-snapshots unit_name/unit_factor and re-checks
// the active (supplier, product, unit) uniqueness.
func (s *PriceAgreementService) UpdatePriceAgreement(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdatePriceAgreementRequest],
) (*connect.Response[inventoryifacev1.UpdatePriceAgreementResponse], error) {
	pa, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	unitID := strings.TrimSpace(req.Msg.ProductUnitId)
	if unitID == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "price_agreement.required")
	}
	if req.Msg.Price < 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "price_agreement.price_invalid")
	}
	validFrom, err := parseDate(req.Msg.ValidFrom)
	if err != nil {
		return nil, err
	}
	validUntil, err := parseDate(req.Msg.ValidUntil)
	if err != nil {
		return nil, err
	}
	if validFrom != nil && validUntil != nil && validUntil.Before(*validFrom) {
		return nil, common.TokenError(connect.CodeInvalidArgument, "price_agreement.bad_dates")
	}

	db := s.db.WithContext(ctx)
	unit, err := resolveUnit(ctx, db, pa.ProductID, unitID)
	if err != nil {
		return nil, err
	}
	if pa.Active {
		if taken, e := common.ExistsBy(db, &model.PriceAgreement{},
			"supplier_id = ? AND product_id = ? AND product_unit_id = ? AND active AND id <> ?",
			pa.SupplierID, pa.ProductID, unitID, pa.ID); e != nil {
			return nil, connect.NewError(connect.CodeInternal, e)
		} else if taken {
			return nil, common.TokenError(connect.CodeAlreadyExists, "price_agreement.exists")
		}
	}

	updates := map[string]any{
		"product_unit_id": unitID,
		"unit_name":       unit.Name,
		"unit_factor":     unit.Factor,
		"price":           req.Msg.Price,
		"valid_from":      validFrom,
		"valid_until":     validUntil,
		"note":            strings.TrimSpace(req.Msg.Note),
	}
	if err := db.Model(pa).Updates(updates).Error; err != nil {
		return nil, common.TokenError(connect.CodeAlreadyExists, "price_agreement.exists")
	}
	// Reload for the fresh snapshot/values in the response.
	pa, err = s.load(ctx, pa.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.UpdatePriceAgreementResponse{Agreement: agreementToProto(pa)}), nil
}
