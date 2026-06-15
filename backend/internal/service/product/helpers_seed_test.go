package product_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	productsvc "github.com/justmart/backend/internal/service/product"
)

// seedProduct creates a product (with its base unit + initial price row) via the
// real CreateProduct handler and returns the created Product id. The caller ctx
// must carry an authenticated principal (CreateProduct FKs caller.UserID into
// product_prices.changed_by).
func seedProduct(t *testing.T, svc *productsvc.ProductService, ctx context.Context, sku, name string, unitPrice int64) string {
	t.Helper()
	resp, err := svc.CreateProduct(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku:       sku,
		Name:      name,
		Unit:      "tablet",
		UnitPrice: unitPrice,
	}))
	require.NoError(t, err)
	return resp.Msg.Product.Id
}

// defaultWarehouseID returns the migration-seeded default warehouse id (MAIN).
func defaultWarehouseID(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var w model.Warehouse
	require.NoError(t, db.Where("is_default").First(&w).Error)
	return w.ID
}

// seedRestockLast inserts a product_last_restocks row (one per warehouse+product
// +supplier) so ListProducts' last_restock enrich + supplier-detail reads have data.
func seedRestockLast(t *testing.T, db *gorm.DB, warehouseID, productID, supplierID string, price, qty int64, arrived time.Time) {
	t.Helper()
	require.NoError(t, db.Create(&model.ProductLastRestock{
		WarehouseID:      warehouseID,
		ProductID:        productID,
		SupplierID:       supplierID,
		LastPrice:        price,
		LastQty:          qty,
		LastDiscountType: "FIXED",
		LastCreatedAt:    arrived.Add(-48 * time.Hour),
		LastArrivedAt:    arrived,
		UpdatedAt:        arrived,
	}).Error)
}

// seedBatchWithStock inserts a batch for the product plus one PURCHASE stock
// movement of `qty` base units in the given warehouse, so stock-aggregating
// reads (GetProduct, ListLowStock) see real on-hand quantity. Returns batch id.
func seedBatchWithStock(t *testing.T, db *gorm.DB, productID, warehouseID, userID string, qty int32, costPrice int64) string {
	t.Helper()
	now := time.Now()
	batch := model.Batch{
		ProductID:   productID,
		BatchNumber: "BATCH-1",
		ExpiryDate:  now.Add(365 * 24 * time.Hour),
		CostPrice:   costPrice,
		ReceivedAt:  now,
	}
	require.NoError(t, db.Create(&batch).Error)
	mv := model.StockMovement{
		BatchID:     batch.ID,
		Qty:         qty,
		Type:        "PURCHASE",
		UserID:      userID,
		WarehouseID: warehouseID,
	}
	require.NoError(t, db.Create(&mv).Error)
	return batch.ID
}
