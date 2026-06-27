package priceagreement

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *PriceAgreementService) load(ctx context.Context, id string) (*model.PriceAgreement, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var pa model.PriceAgreement
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&pa).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("price agreement %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &pa, nil
}

func agreementToProto(p *model.PriceAgreement) *inventoryifacev1.PriceAgreement {
	out := &inventoryifacev1.PriceAgreement{
		Id:            p.ID,
		SupplierId:    p.SupplierID,
		ProductId:     p.ProductID,
		ProductUnitId: p.ProductUnitID,
		UnitName:      p.UnitName,
		UnitFactor:    p.UnitFactor,
		Price:         p.Price,
		ValidFrom:     dateStr(p.ValidFrom),
		ValidUntil:    dateStr(p.ValidUntil),
		Note:          p.Note,
		Active:        p.Active,
		CreatedAt:     p.CreatedAt.Unix(),
	}
	return out
}

func dateStr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(common.DateLayout)
}

// parseDate turns a "YYYY-MM-DD" string into a *time.Time ("" → nil).
func parseDate(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(common.DateLayout, s)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid date %q (want YYYY-MM-DD)", s))
	}
	return &t, nil
}

// buildAgreement validates one (product, unit, price, dates) line against the
// given supplier and returns an UNSAVED PriceAgreement (unit_name/unit_factor
// snapshotted, active=true). It checks product/unit required, price, date
// validity, product existence, unit membership, and the existing-active
// duplicate. The caller validates the supplier itself (once) and, for batch
// creates, detects within-request duplicates. Pass a tx as `db` inside a batch
// so its existence/duplicate checks see uncommitted siblings. Shared by both the
// single CreatePriceAgreement and the batch CreatePriceAgreements so validation
// and tokens stay identical.
func buildAgreement(
	ctx context.Context, db *gorm.DB, supplierID, productID, unitID string,
	price int64, validFromStr, validUntilStr, note string,
) (*model.PriceAgreement, error) {
	productID = strings.TrimSpace(productID)
	unitID = strings.TrimSpace(unitID)
	if productID == "" || unitID == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "price_agreement.required")
	}
	if price < 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "price_agreement.price_invalid")
	}
	validFrom, err := parseDate(validFromStr)
	if err != nil {
		return nil, err
	}
	validUntil, err := parseDate(validUntilStr)
	if err != nil {
		return nil, err
	}
	if validFrom != nil && validUntil != nil && validUntil.Before(*validFrom) {
		return nil, common.TokenError(connect.CodeInvalidArgument, "price_agreement.bad_dates")
	}
	if ok, e := common.ExistsBy(db, &model.Product{}, "id = ?", productID); e != nil {
		return nil, connect.NewError(connect.CodeInternal, e)
	} else if !ok {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "price_agreement.product_missing")
	}
	unit, err := resolveUnit(ctx, db, productID, unitID)
	if err != nil {
		return nil, err
	}
	// One active agreement per (supplier, product, unit) — pre-check for a clean
	// token; the partial unique index backstops the race.
	if taken, e := common.ExistsBy(db, &model.PriceAgreement{},
		"supplier_id = ? AND product_id = ? AND product_unit_id = ? AND active", supplierID, productID, unitID); e != nil {
		return nil, connect.NewError(connect.CodeInternal, e)
	} else if taken {
		return nil, common.TokenError(connect.CodeAlreadyExists, "price_agreement.exists")
	}
	return &model.PriceAgreement{
		SupplierID:    supplierID,
		ProductID:     productID,
		ProductUnitID: unitID,
		UnitName:      unit.Name,
		UnitFactor:    unit.Factor,
		Price:         price,
		ValidFrom:     validFrom,
		ValidUntil:    validUntil,
		Note:          strings.TrimSpace(note),
		Active:        true,
	}, nil
}

// resolveUnit loads the product_unit, asserting it belongs to the product and is
// active, and returns it (factor floored at 1). Reused by Create/Update to
// snapshot unit_name/unit_factor.
func resolveUnit(ctx context.Context, db *gorm.DB, productID, unitID string) (*model.ProductUnit, error) {
	var u model.ProductUnit
	err := db.WithContext(ctx).
		Where("id = ? AND product_id = ? AND active", unitID, productID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("unit not found for this product (or inactive)"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if u.Factor < 1 {
		u.Factor = 1
	}
	return &u, nil
}
