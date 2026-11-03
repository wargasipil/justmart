package manufacturer_test

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
)

// seedProduct inserts a product, optionally linked to a manufacturer. The id is
// passed as a pointer because the column is nullable and NULL (no pabrik) is
// the normal state.
func seedProduct(t *testing.T, db *gorm.DB, sku, name string, mfrID *string, active bool) string {
	t.Helper()
	p := model.Product{
		SKU:            sku,
		Name:           name,
		Unit:           "pcs",
		UnitPrice:      5_000,
		Active:         active,
		ManufacturerID: mfrID,
	}
	require.NoError(t, db.Create(&p).Error)
	// The primary is always a member of the approved-source list — migration
	// 00060 backfilled it and every writer maintains it — so seeding the column
	// alone would build a state the server cannot produce.
	if mfrID != nil && *mfrID != "" {
		require.NoError(t, db.Create(&model.ProductManufacturer{
			ProductID: p.ID, ManufacturerID: *mfrID,
		}).Error)
	}
	if !active {
		// GORM omits a false bool whose column carries `default:true`, so the
		// insert above would have written active = true. Set it explicitly.
		require.NoError(t, db.Model(&p).Update("active", false).Error)
	}
	return p.ID
}

// seedStock gives a product `qty` on hand in the given warehouse, via a batch +
// one PURCHASE movement — the same shape the real receive path writes.
func seedStock(t *testing.T, db *gorm.DB, productID, warehouseID, userID string, qty int32) {
	t.Helper()
	now := time.Now()
	batch := model.Batch{
		ProductID:   productID,
		BatchNumber: "MFR-TEST-1",
		ExpiryDate:  now.Add(365 * 24 * time.Hour),
		CostPrice:   1_000,
		ReceivedAt:  now,
	}
	require.NoError(t, db.Create(&batch).Error)
	require.NoError(t, db.Create(&model.StockMovement{
		BatchID: batch.ID, Qty: qty, Type: "PURCHASE",
		UserID: userID, WarehouseID: warehouseID,
	}).Error)
}

func ownerCtx(t *testing.T, db *gorm.DB) (context.Context, string, string) {
	t.Helper()
	cfg := servicetest.NewConfig(t)
	ownerID := servicetest.EnsureOwner(t, db, cfg)
	var wh model.Warehouse
	require.NoError(t, db.Where("is_default").First(&wh).Error)
	return servicetest.OwnerCtx(context.Background(), ownerID), ownerID, wh.ID
}

// A pabrik's page lists everything it MAY make, not only what it is the USUAL
// maker for. This is the whole reason products.manufacturer_id could not answer
// the question on its own: a generic sourced from three factories belongs on
// all three pages, and one column can name only one of them.
func TestListManufacturerProducts_ListsApprovedNotOnlyPrimary(t *testing.T) {
	t.Parallel()
	svc, db := newEnv(t)
	ctx, _, _ := ownerCtx(t, db)
	kalbe := seedManufacturer(t, svc, "MFR-APR-1", "Kalbe Farma").Id
	dexa := seedManufacturer(t, svc, "MFR-APR-2", "Dexa Medica").Id

	// Usual maker Kalbe, but ALSO approved for Dexa -- the second source a
	// buyer recorded off an invoice.
	prod := seedProduct(t, db, "sku-apr-1", "Paracetamol 500", &kalbe, true)
	require.NoError(t, db.Create(&model.ProductManufacturer{
		ProductID: prod, ManufacturerID: dexa,
	}).Error)

	// It appears on BOTH pages, and Dexa's is the one that used to be empty.
	for _, mfr := range []string{kalbe, dexa} {
		resp, err := svc.ListManufacturerProducts(ctx, connect.NewRequest(
			&inventoryifacev1.ListManufacturerProductsRequest{ManufacturerId: mfr},
		))
		require.NoError(t, err)
		require.Len(t, resp.Msg.Products, 1)
		require.Equal(t, prod, resp.Msg.Products[0].ProductId)
		require.Equal(t, int32(1), resp.Msg.Total)
	}
}

// A product approved for a pabrik it is NOT the usual maker for still belongs
// to exactly one row of that page -- the subquery form is what keeps it from
// being counted twice the way a join would.
func TestListManufacturerProducts_NoDuplicateRowPerApproval(t *testing.T) {
	t.Parallel()
	svc, db := newEnv(t)
	ctx, _, _ := ownerCtx(t, db)
	kalbe := seedManufacturer(t, svc, "MFR-DUP-1", "Kalbe Farma").Id
	prod := seedProduct(t, db, "sku-dup-1", "Amoxsan", &kalbe, true)

	resp, err := svc.ListManufacturerProducts(ctx, connect.NewRequest(
		&inventoryifacev1.ListManufacturerProductsRequest{ManufacturerId: kalbe},
	))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Products, 1, "approved AND primary is still one product")
	require.Equal(t, int32(1), resp.Msg.Total, "total must agree with the page")
	require.Equal(t, prod, resp.Msg.Products[0].ProductId)
}

func TestListManufacturerProducts_OnlyThisMakersProducts(t *testing.T) {
	t.Parallel()
	svc, db := newEnv(t)
	ctx, _, _ := ownerCtx(t, db)
	mine := seedManufacturer(t, svc, "LP-1", "Mine")
	theirs := seedManufacturer(t, svc, "LP-2", "Theirs")
	seedProduct(t, db, "SKU-A", "Alpha", &mine.Id, true)
	seedProduct(t, db, "SKU-B", "Bravo", &theirs.Id, true)
	seedProduct(t, db, "SKU-C", "Charlie", nil, true) // no pabrik recorded

	resp, err := svc.ListManufacturerProducts(ctx,
		connect.NewRequest(&inventoryifacev1.ListManufacturerProductsRequest{ManufacturerId: mine.Id}))
	require.NoError(t, err)
	require.EqualValues(t, 1, resp.Msg.Total)
	require.Len(t, resp.Msg.Products, 1)
	require.Equal(t, "Alpha", resp.Msg.Products[0].Name)
	require.Equal(t, "SKU-A", resp.Msg.Products[0].Sku)
	require.Equal(t, "pcs", resp.Msg.Products[0].BaseUnit)
	require.EqualValues(t, 5_000, resp.Msg.Products[0].UnitPrice)
}

// ready_stock is scoped to the caller's ACTIVE warehouse, like every other
// stock read — so the figure here matches what the product pages show.
func TestListManufacturerProducts_ReadyStockIsWarehouseScoped(t *testing.T) {
	t.Parallel()
	svc, db := newEnv(t)
	ctx, ownerID, mainWH := ownerCtx(t, db)
	mfr := seedManufacturer(t, svc, "LP-WH", "Stocked")
	productID := seedProduct(t, db, "SKU-WH", "Stocked Item", &mfr.Id, true)
	seedStock(t, db, productID, mainWH, ownerID, 40)

	// A second warehouse holding more of the same product must NOT leak in.
	other := model.Warehouse{Code: "WH-2", Name: "Gudang Dua", Active: true}
	require.NoError(t, db.Create(&other).Error)
	seedStock(t, db, productID, other.ID, ownerID, 500)

	resp, err := svc.ListManufacturerProducts(ctx,
		connect.NewRequest(&inventoryifacev1.ListManufacturerProductsRequest{ManufacturerId: mfr.Id}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Products, 1)
	require.EqualValues(t, 40, resp.Msg.Products[0].ReadyStock)
}

func TestListManufacturerProducts_ArchivedHiddenUnlessAsked(t *testing.T) {
	t.Parallel()
	svc, db := newEnv(t)
	ctx, _, _ := ownerCtx(t, db)
	mfr := seedManufacturer(t, svc, "LP-AR", "Has Archived")
	seedProduct(t, db, "SKU-LIVE", "Live", &mfr.Id, true)
	seedProduct(t, db, "SKU-DEAD", "Dead", &mfr.Id, false)

	def, err := svc.ListManufacturerProducts(ctx,
		connect.NewRequest(&inventoryifacev1.ListManufacturerProductsRequest{ManufacturerId: mfr.Id}))
	require.NoError(t, err)
	require.EqualValues(t, 1, def.Msg.Total)

	all, err := svc.ListManufacturerProducts(ctx,
		connect.NewRequest(&inventoryifacev1.ListManufacturerProductsRequest{
			ManufacturerId: mfr.Id, IncludeArchived: true,
		}))
	require.NoError(t, err)
	require.EqualValues(t, 2, all.Msg.Total)
}

func TestListManufacturerProducts_SearchMatchesNameOrSku(t *testing.T) {
	t.Parallel()
	svc, db := newEnv(t)
	ctx, _, _ := ownerCtx(t, db)
	mfr := seedManufacturer(t, svc, "LP-Q", "Searchable")
	seedProduct(t, db, "PCT-500", "Paracetamol", &mfr.Id, true)
	seedProduct(t, db, "AMX-500", "Amoxicillin", &mfr.Id, true)

	byName, err := svc.ListManufacturerProducts(ctx,
		connect.NewRequest(&inventoryifacev1.ListManufacturerProductsRequest{ManufacturerId: mfr.Id, Query: "paracet"}))
	require.NoError(t, err)
	require.EqualValues(t, 1, byName.Msg.Total)

	bySku, err := svc.ListManufacturerProducts(ctx,
		connect.NewRequest(&inventoryifacev1.ListManufacturerProductsRequest{ManufacturerId: mfr.Id, Query: "AMX"}))
	require.NoError(t, err)
	require.EqualValues(t, 1, bySku.Msg.Total)
	require.Equal(t, "Amoxicillin", bySku.Msg.Products[0].Name)
}

// A bad id is a clean NotFound, not an empty page that reads as "this pabrik
// makes nothing".
func TestListManufacturerProducts_UnknownManufacturerIsNotFound(t *testing.T) {
	t.Parallel()
	svc, db := newEnv(t)
	ctx, _, _ := ownerCtx(t, db)

	_, err := svc.ListManufacturerProducts(ctx,
		connect.NewRequest(&inventoryifacev1.ListManufacturerProductsRequest{
			ManufacturerId: "00000000-0000-4000-8000-000000000000",
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
