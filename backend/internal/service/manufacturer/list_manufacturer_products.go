package manufacturer

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ListManufacturerProducts returns the catalog this pabrik makes. It is what
// earns the manufacturer a detail page at all — without it a manufacturer is
// seven scalar fields the edit drawer already renders.
//
// Returns a DENORMALIZED row rather than a full Product on purpose: the table
// is display-only, and Product carries cost fields (reference_cost,
// stock_valuation, last_restock_price…) that would then need redacting on a
// read with no business exposing them. `ready_stock` is active-warehouse-scoped
// like every other stock read in the app.
func (s *ManufacturerService) ListManufacturerProducts(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListManufacturerProductsRequest],
) (*connect.Response[inventoryifacev1.ListManufacturerProductsResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	// Load it first so a bad id is a clean NotFound rather than an empty page.
	m, err := s.load(ctx, req.Msg.ManufacturerId)
	if err != nil {
		return nil, err
	}
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	query := strings.TrimSpace(req.Msg.Query)
	includeArchived := req.Msg.IncludeArchived

	applyFilters := func(q *gorm.DB) *gorm.DB {
		// Everything this pabrik MAY make, not only what it is the USUAL maker
		// for. The approved-source list is the question a pabrik's page asks —
		// a generic sourced from three factories belongs on all three pages,
		// and products.manufacturer_id can only ever name one of them.
		//
		// Written as a subquery rather than a join so the two chains below stay
		// one row per product: a product approved for this maker joins exactly
		// once today, but a join would start double-counting the moment the
		// pair index stopped being unique.
		//
		// The trailing OR on the primary pointer is a safety net, not a second
		// rule: the invariant makes it redundant in healthy data (00060
		// backfilled it, every writer maintains it), and keeping it means this
		// read is a strict SUPERSET of what it returned before — a product
		// cannot drop off a pabrik's page because some future writer forgot
		// the list.
		q = q.Where("id IN (?) OR manufacturer_id = ?",
			q.Session(&gorm.Session{NewDB: true}).
				Model(&model.ProductManufacturer{}).
				Select("product_id").
				Where("manufacturer_id = ?", m.ID),
			m.ID)
		if !includeArchived {
			q = q.Where("active = ?", true)
		}
		if query != "" {
			pattern := "%" + query + "%"
			q = q.Where("name "+common.LikeOp(q)+" ? OR sku "+common.LikeOp(q)+" ?", pattern, pattern)
		}
		return q
	}

	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Product{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rows []model.Product
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.Product{})).
		Order("name, id").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ready, err := s.readyStock(ctx, caller, rows)
	if err != nil {
		return nil, err
	}

	out := make([]*inventoryifacev1.ManufacturerProduct, 0, len(rows))
	for i := range rows {
		p := &rows[i]
		out = append(out, &inventoryifacev1.ManufacturerProduct{
			ProductId:  p.ID,
			Name:       p.Name,
			Sku:        p.SKU,
			BaseUnit:   p.Unit,
			UnitPrice:  p.UnitPrice,
			ReadyStock: ready[p.ID],
			Active:     p.Active,
		})
	}
	return connect.NewResponse(&inventoryifacev1.ListManufacturerProductsResponse{
		Products: out,
		Total:    int32(total),
	}), nil
}

// readyStock batch-loads on-hand per product in the caller's active warehouse —
// one grouped query for the whole page, never N+1. Same join product.enrichStock
// uses, so the number here matches the one the product pages show.
func (s *ManufacturerService) readyStock(
	ctx context.Context,
	caller auth.Principal,
	rows []model.Product,
) (map[string]int64, error) {
	out := make(map[string]int64, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].ID)
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	type readyRow struct {
		ProductID string `gorm:"column:product_id"`
		Qty       int64  `gorm:"column:qty"`
	}
	var readyRows []readyRow
	if err := s.db.WithContext(ctx).
		Table("batches AS b").
		Select("b.product_id AS product_id, COALESCE(SUM(sm.qty), 0) AS qty").
		Joins("LEFT JOIN stock_movements sm ON sm.batch_id = b.id AND sm.warehouse_id = ?", warehouseID).
		Where("b.product_id IN ?", ids).
		Group("b.product_id").Scan(&readyRows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, r := range readyRows {
		out[r.ProductID] = r.Qty
	}
	return out, nil
}
