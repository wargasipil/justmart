package purchasing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

const (
	discountFixed   = "FIXED"
	discountPercent = "PERCENT"
)

// lineNetSubtotal returns the NET extended line amount (gross − line discount)
// and the normalized discount type. discValue is minor units when FIXED, basis
// points (percent*100, so 12.5% = 1250) when PERCENT. The discount is clamped to
// [0, gross] so the net is never negative; PERCENT rounds half-up like PPN.
func lineNetSubtotal(gross int64, discType string, discValue int64) (net int64, normType string, err error) {
	if discValue < 0 {
		return 0, "", connect.NewError(connect.CodeInvalidArgument, errors.New("discount_value must be >= 0"))
	}
	normType = discType
	if normType == "" {
		normType = discountFixed
	}
	var disc int64
	switch normType {
	case discountFixed:
		disc = discValue
	case discountPercent:
		if discValue > 10000 { // 100.00%
			return 0, "", connect.NewError(connect.CodeInvalidArgument, errors.New("percent discount must be within 0..10000 basis points"))
		}
		disc = (gross*discValue + 5000) / 10000 // round half up
	default:
		return 0, "", connect.NewError(connect.CodeInvalidArgument, errors.New("discount_type must be FIXED or PERCENT"))
	}
	if disc < 0 {
		disc = 0
	}
	if disc > gross {
		disc = gross
	}
	return gross - disc, normType, nil
}

// computePOTotals computes PPN-exclusive purchase totals.
func computePOTotals(items []model.PurchaseOrderItem, cartDiscount int64, ppnEnabled bool, ppnRate int32) (subtotal, discount int64, effectiveRate int32, ppn, total int64) {
	for _, it := range items {
		subtotal += it.Subtotal
	}
	if cartDiscount < 0 {
		cartDiscount = 0
	}
	if cartDiscount > subtotal {
		cartDiscount = subtotal
	}
	dpp := subtotal - cartDiscount
	effectiveRate = ppnRate
	if effectiveRate <= 0 {
		effectiveRate = defaultPPNRate
	}
	if effectiveRate > 100 {
		effectiveRate = 100
	}
	if ppnEnabled {
		ppn = (dpp*int64(effectiveRate) + 50) / 100 // round to nearest rupiah
	}
	return subtotal, cartDiscount, effectiveRate, ppn, dpp + ppn
}

func (p *PurchaseOrders) lockByID(tx *gorm.DB, id string) (*model.PurchaseOrder, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var po model.PurchaseOrder
	err := common.RowLock(tx).Where("id = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("purchase order not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &po, nil
}

func (p *PurchaseOrders) loadFull(ctx context.Context, id string) (*model.PurchaseOrder, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var po model.PurchaseOrder
	err := p.db.WithContext(ctx).Preload("Items").Where("id = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("purchase order not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &po, nil
}

func parseDateMaybe(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, fmt.Errorf("date must be YYYY-MM-DD: %w", err)
	}
	return &t, nil
}

func assignPONo(tx *gorm.DB, now time.Time) (string, error) {
	year := now.Year()
	var counter model.POCounter
	err := tx.Where("year = ?", year).First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		counter = model.POCounter{Year: year, LastSeq: 0}
		if err := tx.Create(&counter).Error; err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if err := tx.Model(&model.POCounter{}).
		Where("year = ?", year).
		Update("last_seq", gorm.Expr("last_seq + 1")).Error; err != nil {
		return "", err
	}
	if err := tx.Where("year = ?", year).First(&counter).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("PO-%d-%04d", year, counter.LastSeq), nil
}

func assignReceiptNo(tx *gorm.DB, now time.Time) (string, error) {
	year := now.Year()
	var counter model.RcvCounter
	err := tx.Where("year = ?", year).First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		counter = model.RcvCounter{Year: year, LastSeq: 0}
		if err := tx.Create(&counter).Error; err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if err := tx.Model(&model.RcvCounter{}).
		Where("year = ?", year).
		Update("last_seq", gorm.Expr("last_seq + 1")).Error; err != nil {
		return "", err
	}
	if err := tx.Where("year = ?", year).First(&counter).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("RCV-%d-%04d", year, counter.LastSeq), nil
}

// recomputePOStatus inspects items and bumps po.status accordingly. Caller is
// inside a tx and the po row is locked.
func recomputePOStatus(tx *gorm.DB, po *model.PurchaseOrder) error {
	var items []model.PurchaseOrderItem
	if err := tx.Where("purchase_order_id = ?", po.ID).Find(&items).Error; err != nil {
		return err
	}
	allReceived := true
	anyReceived := false
	for _, it := range items {
		if it.ReceivedQty < it.OrderedQty {
			allReceived = false
		}
		if it.ReceivedQty > 0 {
			anyReceived = true
		}
	}
	var newStatus string
	switch {
	case allReceived && anyReceived:
		newStatus = poStatusReceived
	case anyReceived:
		newStatus = poStatusPartiallyReceived
	default:
		return nil
	}
	if po.Status == poStatusClosed {
		return nil
	}
	po.Status = newStatus
	return tx.Model(po).Update("status", newStatus).Error
}

// maybeCloseIfPaid: when paid >= ordered_total AND status == RECEIVED, mark CLOSED.
func maybeCloseIfPaid(tx *gorm.DB, po *model.PurchaseOrder) error {
	if po.Status == poStatusReceived && po.PaidAmount >= po.OrderedTotal && po.OrderedTotal > 0 {
		now := time.Now()
		po.Status = poStatusClosed
		po.ClosedAt = &now
		return tx.Model(po).Updates(map[string]any{
			"status":    poStatusClosed,
			"closed_at": now,
		}).Error
	}
	return nil
}

// resolvePurchaseUnit returns the product's purchasable unit for the given unit
// id, or its base unit when unitID is empty.
func resolvePurchaseUnit(tx *gorm.DB, productID, unitID string) (*model.ProductUnit, error) {
	var u model.ProductUnit
	q := tx.Where("product_id = ? AND active", productID)
	if unitID != "" {
		q = q.Where("id = ? AND purchasable", unitID)
	} else {
		q = q.Where("is_base")
	}
	if err := q.First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("purchase unit not found or not purchasable"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if u.Factor < 1 {
		u.Factor = 1
	}
	return &u, nil
}

// enrichList denormalizes display-only data onto a page of POs: product names
// for each ordered item, plus the most recent receipt's date + invoice number,
// plus warehouse names. Batched to avoid N+1.
func (p *PurchaseOrders) enrichList(ctx context.Context, orders []*purchasingifacev1.PurchaseOrder) error {
	if len(orders) == 0 {
		return nil
	}
	poIDs := make([]string, 0, len(orders))
	medIDSet := map[string]struct{}{}
	for _, po := range orders {
		poIDs = append(poIDs, po.Id)
		for _, it := range po.Items {
			if it.ProductId != "" {
				medIDSet[it.ProductId] = struct{}{}
			}
		}
	}

	// Product names + SKUs.
	if len(medIDSet) > 0 {
		medIDs := make([]string, 0, len(medIDSet))
		for id := range medIDSet {
			medIDs = append(medIDs, id)
		}
		type nameRow struct {
			ID   string `gorm:"column:id"`
			Name string `gorm:"column:name"`
			Sku  string `gorm:"column:sku"`
		}
		var names []nameRow
		if err := p.db.WithContext(ctx).Table("products").
			Select("id, name, sku").Where("id IN ?", medIDs).Scan(&names).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		nameByID := make(map[string]string, len(names))
		skuByID := make(map[string]string, len(names))
		for _, n := range names {
			nameByID[n.ID] = n.Name
			skuByID[n.ID] = n.Sku
		}
		for _, po := range orders {
			for _, it := range po.Items {
				it.ProductName = nameByID[it.ProductId]
				it.ProductSku = skuByID[it.ProductId]
			}
		}
	}

	// Most recent receipt per PO (received_at + invoice_no).
	type rcvRow struct {
		PurchaseOrderID string    `gorm:"column:purchase_order_id"`
		ReceivedAt      time.Time `gorm:"column:received_at"`
		InvoiceNo       string    `gorm:"column:invoice_no"`
	}
	var rcvs []rcvRow
	if err := p.db.WithContext(ctx).
		Raw(`SELECT purchase_order_id, received_at, invoice_no FROM (
		        SELECT purchase_order_id, received_at, invoice_no,
		               ROW_NUMBER() OVER (PARTITION BY purchase_order_id ORDER BY received_at DESC, created_at DESC) AS rn
		        FROM purchase_receipts
		        WHERE purchase_order_id IN ?
		     ) t WHERE rn = 1`, poIDs).
		Scan(&rcvs).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	type rcvInfo struct {
		at  int64
		inv string
	}
	rcvByPO := make(map[string]rcvInfo, len(rcvs))
	for _, r := range rcvs {
		rcvByPO[r.PurchaseOrderID] = rcvInfo{at: r.ReceivedAt.Unix(), inv: r.InvoiceNo}
	}
	for _, po := range orders {
		if info, ok := rcvByPO[po.Id]; ok {
			po.ReceivedAt = info.at
			if po.InvoiceNo == "" {
				po.InvoiceNo = info.inv
			}
		}
	}

	// Warehouse names.
	whIDSet := map[string]struct{}{}
	for _, po := range orders {
		if po.WarehouseId != "" {
			whIDSet[po.WarehouseId] = struct{}{}
		}
	}
	if len(whIDSet) > 0 {
		whIDs := make([]string, 0, len(whIDSet))
		for id := range whIDSet {
			whIDs = append(whIDs, id)
		}
		type whRow struct {
			ID   string `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		var rows []whRow
		if err := p.db.WithContext(ctx).Table("warehouses").
			Select("id, name").Where("id IN ?", whIDs).Scan(&rows).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		byID := make(map[string]string, len(rows))
		for _, r := range rows {
			byID[r.ID] = r.Name
		}
		for _, po := range orders {
			po.WarehouseName = byID[po.WarehouseId]
		}
	}
	return nil
}

func (p *PurchaseReceipts) loadFull(ctx context.Context, id string) (*model.PurchaseReceipt, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var r model.PurchaseReceipt
	err := p.db.WithContext(ctx).Preload("Items").Where("id = ?", id).First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("receipt not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &r, nil
}
