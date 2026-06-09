package common

import (
	"context"

	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
)

// DateLayout is the canonical YYYY-MM-DD format used for date-only columns
// (batch expiry / received dates, opname filters).
const DateLayout = "2006-01-02"

// BatchCurrentQty returns SUM(stock_movements.qty) for a batch across ALL
// warehouses (the global lot total). Used where location is irrelevant.
func BatchCurrentQty(ctx context.Context, db *gorm.DB, batchID string) (int64, error) {
	var total *int64
	err := db.WithContext(ctx).
		Model(&model.StockMovement{}).
		Where("batch_id = ?", batchID).
		Select("COALESCE(SUM(qty), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	if total == nil {
		return 0, nil
	}
	return *total, nil
}

// LockBatchesByID takes a FOR UPDATE lock on the given batch rows in a
// deterministic order (by id) so concurrent stock-mutating txs serialize per lot
// without deadlocking (classic ordered locking). Call inside a tx before
// reading/consuming a batch's stock; the batches row acts as the per-lot mutex
// over the insert-only stock_movements ledger. No-op on empty input.
func LockBatchesByID(tx *gorm.DB, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	var dump []model.Batch
	return RowLock(tx).
		Where("id IN ?", ids).
		Order("id").
		Find(&dump).Error
}

// LockBatchesByProduct FOR UPDATE-locks every batch (lot) of the given products
// in deterministic id order. Used by CompleteSale to serialize FEFO consumption
// of a product's lots across concurrent sales. No-op on empty input.
func LockBatchesByProduct(tx *gorm.DB, productIDs []string) error {
	if len(productIDs) == 0 {
		return nil
	}
	var dump []model.Batch
	return RowLock(tx).
		Where("product_id IN ?", productIDs).
		Order("id").
		Find(&dump).Error
}

// BatchQtyInWarehouse returns SUM(qty) for a batch within one warehouse.
// This is the per-location stock figure that POS FEFO and transfers consume.
func BatchQtyInWarehouse(ctx context.Context, db *gorm.DB, batchID, warehouseID string) (int64, error) {
	var total *int64
	err := db.WithContext(ctx).
		Model(&model.StockMovement{}).
		Where("batch_id = ? AND warehouse_id = ?", batchID, warehouseID).
		Select("COALESCE(SUM(qty), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	if total == nil {
		return 0, nil
	}
	return *total, nil
}
