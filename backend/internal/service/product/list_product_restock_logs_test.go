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
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestListProductRestockLogs_WarehouseScopedAndPaged(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	mainWH := defaultWarehouseID(t, gormDB)
	otherWH := seedWarehouseRow(t, gormDB, "WH-RL", "Other WH")
	prodID := seedProduct(t, svc, ctx, "rl-sku", "RL product", 1000)
	supID := seedSupplierRow(t, gormDB, "SUP-RL", "RL supplier")

	// 3 logs in MAIN (ascending arrival) + 1 in another warehouse.
	seedRestockLog(t, gormDB, mainWH, prodID, supID, 900, 5, date(2026, 1, 3))
	seedRestockLog(t, gormDB, mainWH, prodID, supID, 950, 6, date(2026, 2, 3))
	seedRestockLog(t, gormDB, mainWH, prodID, supID, 980, 7, date(2026, 3, 3))
	seedRestockLog(t, gormDB, otherWH, prodID, supID, 800, 9, date(2026, 4, 3))

	resp, err := svc.ListProductRestockLogs(ctx, connect.NewRequest(&inventoryifacev1.ListProductRestockLogsRequest{
		ProductId: prodID,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(3), resp.Msg.Total) // other-warehouse row excluded
	require.Len(t, resp.Msg.Logs, 3)
	// Newest arrival first.
	require.Equal(t, int64(980), resp.Msg.Logs[0].Price)
	require.Equal(t, int64(950), resp.Msg.Logs[1].Price)
	require.Equal(t, int64(900), resp.Msg.Logs[2].Price)
	require.Equal(t, supID, resp.Msg.Logs[0].SupplierId)

	// Pagination: first page of 2.
	page, err := svc.ListProductRestockLogs(ctx, connect.NewRequest(&inventoryifacev1.ListProductRestockLogsRequest{
		ProductId: prodID, Limit: 2, Offset: 0,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(3), page.Msg.Total)
	require.Len(t, page.Msg.Logs, 2)
}

func TestListProductRestockLogs_ProductIDRequired(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.ListProductRestockLogs(ctx, connect.NewRequest(&inventoryifacev1.ListProductRestockLogsRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// --- shared restock-test seed helpers (used by this file) ---

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

func seedWarehouseRow(t *testing.T, db *gorm.DB, code, name string) string {
	t.Helper()
	w := model.Warehouse{Code: code, Name: name}
	require.NoError(t, db.Create(&w).Error)
	return w.ID
}

func seedSupplierRow(t *testing.T, db *gorm.DB, code, name string) string {
	t.Helper()
	s := model.Supplier{Code: code, Name: name, Active: true}
	require.NoError(t, db.Create(&s).Error)
	return s.ID
}

func seedRestockLog(t *testing.T, db *gorm.DB, warehouseID, productID, supplierID string, price, qty int64, arrived time.Time) {
	t.Helper()
	require.NoError(t, db.Create(&model.ProductRestockLog{
		WarehouseID:      warehouseID,
		ProductID:        productID,
		SupplierID:       supplierID,
		Price:            price,
		Qty:              qty,
		DiscountType:     "FIXED",
		RestockCreatedAt: arrived.Add(-48 * time.Hour),
		RestockArrivedAt: arrived,
	}).Error)
}
