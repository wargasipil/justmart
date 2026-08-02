package table

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ListTables returns one page of the ACTIVE WAREHOUSE's floor, ordered by area
// then code (how a floor plan reads) with an id tiebreaker so paging can't drop
// or duplicate a table.
//
// `occupied` counts occupied tables across ALL matches, not just the page — it
// drives a "12/20 occupied" header, which a page-local count would understate on
// every page but the last.
func (s *TableService) ListTables(
	ctx context.Context,
	req *connect.Request[tableifacev1.ListTablesRequest],
) (*connect.Response[tableifacev1.ListTablesResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	area := strings.TrimSpace(req.Msg.Area)

	// One filter closure feeds the count, the page and the occupancy tally, so
	// the three can't drift.
	applyFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Model(&model.DiningTable{}).Where("warehouse_id = ?", warehouseID)
		if !req.Msg.IncludeInactive {
			q = q.Where("active")
		}
		if area != "" {
			q = q.Where("area = ?", area)
		}
		return q
	}

	var total int64
	if err := applyFilters(s.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Occupancy over the whole match set. A correlated EXISTS keeps this one
	// query regardless of floor size.
	occupiedSub := s.db.Session(&gorm.Session{NewDB: true}).
		Table("sales").
		Select("1").
		Where("sales.table_id = dining_tables.id AND sales.status = ?", common.SaleStatusDraft)
	var occupied int64
	if err := applyFilters(s.db.WithContext(ctx)).
		Where("EXISTS (?)", occupiedSub).Count(&occupied).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	page := applyFilters(s.db.WithContext(ctx))
	if req.Msg.OnlyOccupied {
		page = page.Where("EXISTS (?)", occupiedSub)
	}
	var rows []model.DiningTable
	if err := page.
		Order("area ASC, code ASC, id ASC").
		Offset(offset).Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*tableifacev1.DiningTable, len(rows))
	ids := make([]string, len(rows))
	for i := range rows {
		out[i] = toProto(&rows[i])
		ids[i] = rows[i].ID
	}
	bills, err := loadOpenBills(ctx, s.db, ids)
	if err != nil {
		return nil, err
	}
	for _, t := range out {
		applyOccupancy(t, bills)
	}

	// only_occupied narrows the page but `total` stays the unfiltered count for
	// the same filters — the pager's denominator, matching every other List here.
	return connect.NewResponse(&tableifacev1.ListTablesResponse{
		Tables:   out,
		Total:    int32(total),
		Occupied: int32(occupied),
	}), nil
}
