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
