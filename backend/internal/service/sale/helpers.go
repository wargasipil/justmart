package sale

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// applySaleFilters applies the order-history filters (date range, status,
// free-text search) shared by ListSales and GetSalesSummary so the paginated
// list and its summary always agree. The caller's query root must be `sales`.
func (s *SaleService) applySaleFilters(
	q *gorm.DB,
	warehouseID string,
	fromUnix, toUnix int64,
	status posifacev1.SaleStatus,
	query string,
) *gorm.DB {
	if warehouseID != "" {
		q = q.Where("warehouse_id = ?", warehouseID)
	}
	if fromUnix > 0 {
		q = q.Where("created_at >= ?", time.Unix(fromUnix, 0))
	}
	if toUnix > 0 {
		q = q.Where("created_at < ?", time.Unix(toUnix, 0))
	}
	if statusStr := saleStatusToString(status); statusStr != "" {
		q = q.Where("status = ?", statusStr)
	} else {
		// "All" in order history means finalized orders only — in-progress
		// carts (DRAFT) are never shown in history or its summary.
		q = q.Where("status <> ?", saleStatusDraft)
	}
	if qstr := strings.TrimSpace(query); qstr != "" {
		pattern := "%" + qstr + "%"
		sub := s.db.Table("sales AS s").Select("s.id").
			Joins("LEFT JOIN customers c ON c.id = s.customer_id").
			Joins("LEFT JOIN sale_items si ON si.sale_id = s.id").
			Joins("LEFT JOIN products m ON m.id = si.product_id").
			Where("s.sale_no "+common.LikeOp(s.db)+" ? OR c.name "+common.LikeOp(s.db)+" ? OR m.name "+common.LikeOp(s.db)+" ?", pattern, pattern, pattern)
		q = q.Where("id IN (?)", sub)
	}
	return q
}

// enrichSaleNames denormalizes customer + product names onto a page of sales
// for the order-history list (two batched queries, no N+1).
func (s *SaleService) enrichSaleNames(ctx context.Context, sales []*posifacev1.Sale) error {
	if len(sales) == 0 {
		return nil
	}
	custIDSet := map[string]struct{}{}
	medIDSet := map[string]struct{}{}
	for _, sl := range sales {
		if sl.CustomerId != "" {
			custIDSet[sl.CustomerId] = struct{}{}
		}
		for _, it := range sl.Items {
			if it.ProductId != "" {
				medIDSet[it.ProductId] = struct{}{}
			}
		}
	}
	nameByID := func(table string, idset map[string]struct{}) (map[string]string, error) {
		out := map[string]string{}
		if len(idset) == 0 {
			return out, nil
		}
		ids := make([]string, 0, len(idset))
		for id := range idset {
			ids = append(ids, id)
		}
		type row struct {
			ID   string `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		var rows []row
		if err := s.db.WithContext(ctx).Table(table).Select("id, name").
			Where("id IN ?", ids).Scan(&rows).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, r := range rows {
			out[r.ID] = r.Name
		}
		return out, nil
	}
	custNames, err := nameByID("customers", custIDSet)
	if err != nil {
		return err
	}
	medNames, err := nameByID("products", medIDSet)
	if err != nil {
		return err
	}
	for _, sl := range sales {
		sl.CustomerName = custNames[sl.CustomerId]
		for _, it := range sl.Items {
			it.ProductName = medNames[it.ProductId]
		}
	}
	return nil
}

func (s *SaleService) draftForUpdate(tx *gorm.DB, id string) (*model.Sale, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("sale_id required"))
	}
	var sale model.Sale
	err := common.RowLock(tx).Where("id = ?", id).First(&sale).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("sale not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if sale.Status != saleStatusDraft {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("sale is %s; only DRAFT sales accept mutations", sale.Status))
	}
	return &sale, nil
}

func (s *SaleService) loadFull(ctx context.Context, id string) (*model.Sale, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("sale_id required"))
	}
	var sale model.Sale
	err := s.db.WithContext(ctx).Preload("Items").Where("id = ?", id).First(&sale).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("sale not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &sale, nil
}

func recomputeSaleTotals(tx *gorm.DB, saleID string) error {
	var subtotal int64
	if err := tx.Model(&model.SaleItem{}).
		Where("sale_id = ?", saleID).
		Select("COALESCE(SUM(line_total), 0)").
		Scan(&subtotal).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	var current model.Sale
	if err := tx.Where("id = ?", saleID).First(&current).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	// total = items subtotal − cart discount + biaya jasa (service fee). The fee
	// is a sale-level charge snapshotted from the attached resep (editable at POS).
	total := subtotal - current.CartDiscount + current.BiayaJasa
	if total < 0 {
		total = 0
	}
	return tx.Model(&current).Updates(map[string]any{
		"subtotal": subtotal,
		"total":    total,
	}).Error
}

func computeLineTotal(qty int32, unitPrice, lineDiscount int64) int64 {
	gross := int64(qty) * unitPrice
	net := gross - lineDiscount
	if net < 0 {
		return 0
	}
	return net
}

func assignSaleNo(tx *gorm.DB, now time.Time) (string, error) {
	year := now.Year()
	var counter model.SaleNoCounter
	err := tx.Where("year = ?", year).First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		counter = model.SaleNoCounter{Year: year, LastSeq: 0}
		if err := tx.Create(&counter).Error; err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if err := tx.Model(&model.SaleNoCounter{}).
		Where("year = ?", year).
		Update("last_seq", gorm.Expr("last_seq + 1")).Error; err != nil {
		return "", err
	}
	if err := tx.Where("year = ?", year).First(&counter).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("INV-%d-%04d", year, counter.LastSeq), nil
}

// resolveSellUnit returns the product's selling unit for the given unit id, or
// its base unit when unitID is empty. Errors if the unit isn't sellable/active.
func resolveSellUnit(tx *gorm.DB, productID, unitID string) (*model.ProductUnit, error) {
	var u model.ProductUnit
	q := tx.Where("product_id = ? AND active", productID)
	if unitID != "" {
		q = q.Where("id = ? AND sellable", unitID)
	} else {
		q = q.Where("is_base")
	}
	if err := q.First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("selling unit not found or not sellable"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if u.Factor < 1 {
		u.Factor = 1
	}
	return &u, nil
}
