package productdiscount

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

const (
	discountFixed   = "FIXED"
	discountPercent = "PERCENT"
)

func (s *ProductDiscountService) load(ctx context.Context, id string) (*model.ProductDiscount, error) {
	if strings.TrimSpace(id) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var d model.ProductDiscount
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("product discount %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &d, nil
}

func toProto(d *model.ProductDiscount) *inventoryifacev1.ProductDiscount {
	out := &inventoryifacev1.ProductDiscount{
		Id:               d.ID,
		ProductId:        d.ProductID,
		DiscountType:     d.DiscountType,
		PerItem:          d.PerItem,
		Value:            d.Value,
		MinQty:           d.MinQty,
		MinQtyUnitName:   d.MinQtyUnitName,
		MinQtyUnitFactor: d.MinQtyUnitFactor,
		ExpiresAt:        dateStr(d.ExpiresAt),
		CreatedAt:        d.CreatedAt.Unix(),
	}
	if d.MinQtyUnitID != nil {
		out.MinQtyUnitId = *d.MinQtyUnitID
	}
	if out.MinQtyUnitFactor < 1 {
		out.MinQtyUnitFactor = 1
	}
	return out
}

// resolveMinQtyUnit resolves the optional threshold unit: "" → base (nil, "", 1);
// otherwise the unit must belong to the product and be active (mirrors the
// priceagreement resolveUnit). Returns (id, name, factor).
func resolveMinQtyUnit(ctx context.Context, db *gorm.DB, productID, unitID string) (*string, string, int64, error) {
	unitID = strings.TrimSpace(unitID)
	if unitID == "" {
		return nil, "", 1, nil
	}
	var u model.ProductUnit
	err := db.WithContext(ctx).
		Where("id = ? AND product_id = ? AND active", unitID, productID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", 0, common.TokenError(connect.CodeFailedPrecondition, "product_discount.unit_invalid")
	}
	if err != nil {
		return nil, "", 0, connect.NewError(connect.CodeInternal, err)
	}
	if u.Factor < 1 {
		u.Factor = 1
	}
	id := u.ID
	return &id, u.Name, u.Factor, nil
}

// validateDiscount checks the type/value/min-qty and returns the normalized type
// ("" → PERCENT). value is minor units when FIXED, basis points when PERCENT.
func validateDiscount(discType string, value int64, minQty int32) (string, error) {
	if value < 0 {
		return "", common.TokenError(connect.CodeInvalidArgument, "product_discount.value_invalid")
	}
	if minQty < 0 {
		return "", common.TokenError(connect.CodeInvalidArgument, "product_discount.threshold_invalid")
	}
	t := strings.TrimSpace(discType)
	if t == "" {
		t = discountPercent
	}
	switch t {
	case discountFixed:
	case discountPercent:
		if value > 10000 { // > 100.00%
			return "", common.TokenError(connect.CodeInvalidArgument, "product_discount.value_invalid")
		}
	default:
		return "", common.TokenError(connect.CodeInvalidArgument, "product_discount.type_invalid")
	}
	return t, nil
}

func productExists(db *gorm.DB, productID string) error {
	if strings.TrimSpace(productID) == "" {
		return common.TokenError(connect.CodeInvalidArgument, "product_discount.product_missing")
	}
	ok, err := common.ExistsBy(db, &model.Product{}, "id = ?", productID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return common.TokenError(connect.CodeFailedPrecondition, "product_discount.product_missing")
	}
	return nil
}

func dateStr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(common.DateLayout)
}

// parseExpiry turns a "YYYY-MM-DD" string into a *time.Time ("" → nil), emitting
// product_discount.bad_expiry on a parse failure.
func parseExpiry(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(common.DateLayout, s)
	if err != nil {
		return nil, common.TokenError(connect.CodeInvalidArgument, "product_discount.bad_expiry")
	}
	return &t, nil
}
