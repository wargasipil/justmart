package batch

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// ResolveBatches returns minimal display refs (batch number + product name) for
// a set of ids. Unknown ids are omitted; empty input returns an empty list.
func (s *BatchService) ResolveBatches(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ResolveBatchesRequest],
) (*connect.Response[inventoryifacev1.ResolveBatchesResponse], error) {
	ids := common.DedupeIDs(req.Msg.Ids)
	if len(ids) == 0 {
		return connect.NewResponse(&inventoryifacev1.ResolveBatchesResponse{}), nil
	}
	type row struct {
		ID          string `gorm:"column:id"`
		BatchNumber string `gorm:"column:batch_number"`
		ProductID   string `gorm:"column:product_id"`
		ProductName string `gorm:"column:product_name"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Table("batches AS bt").
		Select("bt.id, bt.batch_number, bt.product_id, m.name AS product_name").
		Joins("JOIN products m ON m.id = bt.product_id").
		Where("bt.id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.BatchRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, &inventoryifacev1.BatchRef{
			Id:          r.ID,
			BatchNumber: r.BatchNumber,
			ProductId:   r.ProductID,
			ProductName: r.ProductName,
		})
	}
	return connect.NewResponse(&inventoryifacev1.ResolveBatchesResponse{Batches: out}), nil
}
