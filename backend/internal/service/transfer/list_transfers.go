package transfer

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *TransferService) ListTransfers(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.ListTransfersRequest],
) (*connect.Response[warehouseifacev1.ListTransfersResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	// Scope to the active warehouse — transfers touching it (from OR to). An
	// explicit warehouse_id in the request overrides; otherwise resolve from the
	// X-Warehouse-Id header like every other inventory read.
	wh := req.Msg.WarehouseId
	if wh == "" {
		wh, err = common.ResolveWarehouse(ctx, s.db, caller)
		if err != nil {
			return nil, err
		}
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Where("from_warehouse_id = ? OR to_warehouse_id = ?", wh, wh)
		if query := strings.TrimSpace(req.Msg.Query); query != "" {
			pattern := "%" + query + "%"
			q = q.Where("transfer_no "+common.LikeOp(q)+" ? OR note "+common.LikeOp(q)+" ?", pattern, pattern)
		}
		if req.Msg.FromUnix > 0 {
			q = q.Where("created_at >= ?", time.Unix(req.Msg.FromUnix, 0))
		}
		if req.Msg.ToUnix > 0 {
			q = q.Where("created_at < ?", time.Unix(req.Msg.ToUnix, 0))
		}
		return q
	}
	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.StockTransfer{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.StockTransfer
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.StockTransfer{})).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*warehouseifacev1.StockTransfer, 0, len(rows))
	for i := range rows {
		hydrated, err := s.hydrateTransfer(ctx, &rows[i], false)
		if err != nil {
			return nil, err
		}
		out = append(out, hydrated)
	}
	return connect.NewResponse(&warehouseifacev1.ListTransfersResponse{
		Transfers: out,
		Total:     int32(total),
	}), nil
}
