package supplier_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/servicetest"
	suppliersvc "github.com/justmart/backend/internal/service/supplier"
)

func TestListSupplierRestocks_WarehouseScopedPerProduct(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := suppliersvc.NewSupplierService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	mainWH := defaultWarehouseID(t, gormDB)
	otherWH := warehouseRow(t, gormDB, "WH-SR", "Other WH")
	sup := seedSupplier(t, svc, "SR-SUP", "SR supplier")
	otherSup := supplierRow(t, gormDB, "SR-SUP2", "Other supplier")
	p1 := productRow(t, gormDB, "sr-p1", "Prod 1")
	p2 := productRow(t, gormDB, "sr-p2", "Prod 2")

	// sup: last restocks of p1 (Jan) + p2 (Feb) in MAIN; one in another warehouse;
	// one for another supplier — both excluded.
	lastRestockRow(t, gormDB, mainWH, p1, sup.Id, 900, 5, restockDate(2026, 1, 3))
	lastRestockRow(t, gormDB, mainWH, p2, sup.Id, 700, 8, restockDate(2026, 2, 3))
	lastRestockRow(t, gormDB, otherWH, p1, sup.Id, 600, 3, restockDate(2026, 3, 3))
	lastRestockRow(t, gormDB, mainWH, p1, otherSup, 500, 2, restockDate(2026, 4, 3))

	resp, err := svc.ListSupplierRestocks(ctx, connect.NewRequest(&inventoryifacev1.ListSupplierRestocksRequest{
		SupplierId: sup.Id,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(2), resp.Msg.Total) // other-warehouse + other-supplier excluded
	require.Len(t, resp.Msg.Restocks, 2)
	// Newest arrival first → p2 (Feb) before p1 (Jan).
	require.Equal(t, p2, resp.Msg.Restocks[0].ProductId)
	require.Equal(t, int64(700), resp.Msg.Restocks[0].LastPrice)
	require.Equal(t, int64(8), resp.Msg.Restocks[0].LastQty)
	require.Equal(t, p1, resp.Msg.Restocks[1].ProductId)
}

func TestListSupplierRestocks_SupplierIDRequired(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := suppliersvc.NewSupplierService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	_, err := svc.ListSupplierRestocks(ctx, connect.NewRequest(&inventoryifacev1.ListSupplierRestocksRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// --- local seed helpers ---

func defaultWarehouseID(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var w model.Warehouse
	require.NoError(t, db.Where("is_default").First(&w).Error)
	return w.ID
}

func warehouseRow(t *testing.T, db *gorm.DB, code, name string) string {
	t.Helper()
	w := model.Warehouse{Code: code, Name: name}
	require.NoError(t, db.Create(&w).Error)
	return w.ID
}

func supplierRow(t *testing.T, db *gorm.DB, code, name string) string {
	t.Helper()
	s := model.Supplier{Code: code, Name: name, Active: true}
	require.NoError(t, db.Create(&s).Error)
	return s.ID
}

func productRow(t *testing.T, db *gorm.DB, sku, name string) string {
	t.Helper()
	p := model.Product{SKU: sku, Name: name, Unit: "tablet", UnitPrice: 1000, Active: true}
	require.NoError(t, db.Create(&p).Error)
	return p.ID
}

func restockDate(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

func lastRestockRow(t *testing.T, db *gorm.DB, warehouseID, productID, supplierID string, price, qty int64, arrived time.Time) {
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
