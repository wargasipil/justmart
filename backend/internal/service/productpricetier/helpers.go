package productpricetier

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// minTierQty is the smallest allowed threshold. A tier at 0 or 1 would just be
// the unit's sell_price, and since sale_items.tier_min_qty = 0 is the "no
// grosir" flag, allowing it would let one bad row silently suppress every
// automatic product discount for that product. Mirrored by a DB CHECK.
const minTierQty = 2

// lockProduct serializes a product's tier writes on its product row, the same
// mutex UpdateProduct takes for the unit-price versioning. Without it two
// concurrent edits can both close the open history row and both insert a new
// one, colliding on product_tier_prices_open_idx. No-op on SQLite (single-writer
// pool).
func lockProduct(tx *gorm.DB, productID string) error {
	return common.RowLock(tx).Where("id = ?", productID).First(&model.Product{}).Error
}

// wrapTxError passes a token/connect error from inside a transaction through
// unchanged (the frontend maps those to a field) and wraps anything else as
// Internal.
func wrapTxError(err error) error {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return err
	}
	return connect.NewError(connect.CodeInternal, err)
}

// The rung versioning itself lives in common (CloseTierRung / RecordTierPrice)
// because the `discount-to-grosir` CLI creates tiers too and must open their
// history the same way.

func (s *ProductPriceTierService) load(ctx context.Context, id string) (*model.ProductPriceTier, error) {
	if strings.TrimSpace(id) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var t model.ProductPriceTier
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("product price tier %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &t, nil
}

func toProto(t *model.ProductPriceTier) *inventoryifacev1.ProductPriceTier {
	factor := t.UnitFactor
	if factor < 1 {
		factor = 1
	}
	return &inventoryifacev1.ProductPriceTier{
		Id:            t.ID,
		ProductId:     t.ProductID,
		ProductUnitId: t.ProductUnitID,
		UnitName:      t.UnitName,
		UnitFactor:    factor,
		MinQty:        t.MinQty,
		Price:         t.Price,
		CreatedAt:     t.CreatedAt.Unix(),
	}
}

// resolveTierUnit resolves the unit this tier prices. Unlike the discount
// domain's optional threshold unit, this is REQUIRED — a tier is a price for one
// specific unit, so there is no "" = base shorthand. The unit must belong to the
// product and be active.
func resolveTierUnit(ctx context.Context, db *gorm.DB, productID, unitID string) (*model.ProductUnit, error) {
	unitID = strings.TrimSpace(unitID)
	if unitID == "" {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "product_price_tier.unit_invalid")
	}
	var u model.ProductUnit
	err := db.WithContext(ctx).
		Where("id = ? AND product_id = ? AND active", unitID, productID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "product_price_tier.unit_invalid")
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if u.Factor < 1 {
		u.Factor = 1
	}
	return &u, nil
}

// validateTier checks the threshold and price. The tier price is NOT compared to
// the unit's sell_price here: sell_price legitimately moves later, so a tier that
// is currently not cheaper is stored and simply never applied at POS.
func validateTier(minQty int32, price int64) error {
	if minQty < minTierQty {
		return common.TokenError(connect.CodeInvalidArgument, "product_price_tier.min_qty_invalid")
	}
	if price < 0 {
		return common.TokenError(connect.CodeInvalidArgument, "product_price_tier.price_invalid")
	}
	return nil
}

func productExists(db *gorm.DB, productID string) error {
	if strings.TrimSpace(productID) == "" {
		return common.TokenError(connect.CodeInvalidArgument, "product_price_tier.product_missing")
	}
	ok, err := common.ExistsBy(db, &model.Product{}, "id = ?", productID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return common.TokenError(connect.CodeFailedPrecondition, "product_price_tier.product_missing")
	}
	return nil
}

// assertTierFree pre-checks the (product_unit_id, min_qty) unique index so the
// caller gets a specific token instead of a raw constraint violation. excludeID
// lets Update skip its own row. The unique index backstops the race with the
// same token.
func assertTierFree(db *gorm.DB, unitID string, minQty int32, excludeID string) error {
	q := db.Model(&model.ProductPriceTier{}).
		Where("product_unit_id = ? AND min_qty = ?", unitID, minQty)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if n > 0 {
		return common.TokenError(connect.CodeAlreadyExists, "product_price_tier.tier_taken")
	}
	return nil
}
