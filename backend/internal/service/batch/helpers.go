package batch

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

func (s *BatchService) load(ctx context.Context, id string) (*model.Batch, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var batch model.Batch
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("batch %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &batch, nil
}

// attachUnits batch-loads each batch's owning-product active units (base first,
// then by factor) and sets them on the protos. No N+1 — one query for the page.
// Lets a picker offer per-line unit entry (e.g. transfer "2 box"); stock stays
// in base units. Mirrors product.attachUnits, scoped to the batches' products.
func (s *BatchService) attachUnits(ctx context.Context, batches []*inventoryifacev1.Batch) error {
	if len(batches) == 0 {
		return nil
	}
	idSet := make(map[string]struct{}, len(batches))
	for _, b := range batches {
		if b.ProductId != "" {
			idSet[b.ProductId] = struct{}{}
		}
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	var rows []model.ProductUnit
	if err := s.db.WithContext(ctx).
		Where("product_id IN ? AND active", ids).
		Order("is_base DESC, factor ASC").
		Find(&rows).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	byProduct := make(map[string][]*inventoryifacev1.ProductUnit, len(ids))
	for i := range rows {
		byProduct[rows[i].ProductID] = append(byProduct[rows[i].ProductID], productUnitToProto(&rows[i]))
	}
	for _, b := range batches {
		b.Units = byProduct[b.ProductId]
	}
	return nil
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

func batchToProto(b *model.Batch, qty int64) *inventoryifacev1.Batch {
	out := &inventoryifacev1.Batch{
		Id:              b.ID,
		ProductId:       b.ProductID,
		BatchNumber:     b.BatchNumber,
		ExpiryDate:      b.ExpiryDate.Format(common.DateLayout),
		CostPrice:       b.CostPrice,
		ReceivedAt:      b.ReceivedAt.Format(common.DateLayout),
		CurrentQuantity: qty,
		CreatedAt:       b.CreatedAt.Unix(),
	}
	if b.SupplierID != nil {
		out.SupplierId = *b.SupplierID
	}
	return out
}
