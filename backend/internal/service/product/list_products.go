package product

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *ProductService) ListProducts(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListProductsRequest],
) (*connect.Response[inventoryifacev1.ListProductsResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	query := strings.TrimSpace(req.Msg.Query)
	opnameBefore := strings.TrimSpace(req.Msg.OpnameBefore)
	if opnameBefore != "" {
		if _, perr := time.Parse(common.DateLayout, opnameBefore); perr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("opname_before must be YYYY-MM-DD: %w", perr))
		}
	}
	// Resolve once — both the opname filter (active-warehouse-scoped) and the
	// downstream enrichStock use the same warehouse.
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}

	applyFilters := func(q *gorm.DB) *gorm.DB {
		if req.Msg.OnlyArchived {
			q = q.Where("active = ?", false)
		} else if !req.Msg.IncludeInactive {
			q = q.Where("active = ?", true)
		}
		if query != "" {
			pattern := "%" + query + "%"
			q = q.Where("name "+common.LikeOp(q)+" ? OR sku "+common.LikeOp(q)+" ?", pattern, pattern)
		}
		if opnameBefore != "" {
			// "Last opname < before OR never counted" = "no completed opname
			// session with completed_at >= before touched this product in the
			// active warehouse". Inverted EXISTS keeps both branches.
			q = q.Where(`NOT EXISTS (
				SELECT 1 FROM stocktake_sessions ss
				JOIN stocktake_lines sl ON sl.session_id = ss.id
				JOIN batches b ON b.id = sl.batch_id
				WHERE ss.warehouse_id = ?
				  AND ss.status = 'COMPLETED'
				  AND ss.completed_at >= ?
				  AND sl.counted_qty IS NOT NULL
				  AND b.product_id = products.id
			)`, warehouseID, opnameBefore)
		}
		return q
	}

	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Product{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.Product
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Product{})).
		Order("name").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*inventoryifacev1.Product, 0, len(rows))
	for i := range rows {
		out = append(out, productToProto(&rows[i]))
	}
	if err := s.enrichStock(ctx, caller, out); err != nil {
		return nil, err
	}
	if err := s.enrichLastStocktake(ctx, caller, out); err != nil {
		return nil, err
	}
	if err := s.attachUnits(ctx, out); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.ListProductsResponse{
		Products: out,
		Total:    int32(total),
	}), nil
}
