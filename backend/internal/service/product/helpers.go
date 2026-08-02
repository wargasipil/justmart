package product

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *ProductService) load(ctx context.Context, id string) (*model.Product, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var med model.Product
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&med).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("product %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &med, nil
}

func productToProto(m *model.Product) *inventoryifacev1.Product {
	// 0 = no picture. Set here rather than in an enrich pass so EVERY product
	// read carries it — the list, POS search and detail all key their image
	// fetch off this marker.
	var imageAt int64
	if m.ImageUpdatedAt != nil {
		imageAt = m.ImageUpdatedAt.Unix()
	}
	return &inventoryifacev1.Product{
		Id:                   m.ID,
		Sku:                  m.SKU,
		Name:                 m.Name,
		Unit:                 m.Unit,
		UnitPrice:            m.UnitPrice,
		PrescriptionRequired: m.PrescriptionRequired,
		Active:               m.Active,
		CreatedAt:            m.CreatedAt.Unix(),
		ImageUpdatedAt:       imageAt,
		Kind:                 kindToProto(m.Kind),
	}
}

// kindToProto maps the stored kind string to the proto enum. An empty or
// unrecognized column value reads as STOCKED (never UNSPECIFIED) so a client
// switching on the enum never has to special-case pre-migration rows.
func kindToProto(kind string) inventoryifacev1.ProductKind {
	switch common.NormalizeProductKind(kind) {
	case common.ProductKindComposite:
		return inventoryifacev1.ProductKind_PRODUCT_KIND_COMPOSITE
	case common.ProductKindService:
		return inventoryifacev1.ProductKind_PRODUCT_KIND_SERVICE
	default:
		return inventoryifacev1.ProductKind_PRODUCT_KIND_STOCKED
	}
}

// kindFromProto maps the request enum to the stored string. UNSPECIFIED means
// "the caller didn't say", which is STOCKED — the pre-kind default — so an old
// client that never sets the field keeps creating ordinary stocked products.
func kindFromProto(k inventoryifacev1.ProductKind) string {
	switch k {
	case inventoryifacev1.ProductKind_PRODUCT_KIND_COMPOSITE:
		return common.ProductKindComposite
	case inventoryifacev1.ProductKind_PRODUCT_KIND_SERVICE:
		return common.ProductKindService
	default:
		return common.ProductKindStocked
	}
}

func productUnitToProto(u *model.ProductUnit) *inventoryifacev1.ProductUnit {
	return &inventoryifacev1.ProductUnit{
		Id:          u.ID,
		ProductId:   u.ProductID,
		Name:        u.Name,
		Factor:      u.Factor,
		IsBase:      u.IsBase,
		SellPrice:   u.SellPrice,
		Sellable:    u.Sellable,
		Purchasable: u.Purchasable,
		SortOrder:   int32(u.SortOrder),
		Active:      u.Active,
	}
}

// productPriceTierToProto mirrors productpricetier.toProto — the tier is hydrated
// onto Product here so POS gets the ladder without a second RPC.
func productPriceTierToProto(t *model.ProductPriceTier) *inventoryifacev1.ProductPriceTier {
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

func productPriceToProto(p *model.ProductPrice) *inventoryifacev1.ProductPrice {
	out := &inventoryifacev1.ProductPrice{
		Id:            p.ID,
		ProductId:     p.ProductID,
		UnitPrice:     p.UnitPrice,
		EffectiveFrom: p.EffectiveFrom.Unix(),
		ChangedBy:     p.ChangedBy,
	}
	if p.EffectiveTo != nil {
		out.EffectiveTo = p.EffectiveTo.Unix()
	}
	return out
}
