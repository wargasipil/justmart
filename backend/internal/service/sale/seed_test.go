package sale_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
	salesvc "github.com/justmart/backend/internal/service/sale"
	"github.com/justmart/backend/internal/service/servicetest"
)

// mainWarehouseID is the fixed id of the migration-seeded default warehouse
// ("Gudang Utama", code MAIN). An OWNER with no WarehouseID resolves here via
// common.ResolveWarehouse, so we seed stock into this warehouse to match.
const mainWarehouseID = "00000000-0000-0000-0000-0000000000a1"

// seedProduct inserts a product plus its required base ProductUnit (the unit the
// sale service's resolveSellUnit needs). Stock is NOT seeded — use seedStock for
// that. Returns the product id. Inserts via GORM so the SQLite UUID create-callback
// fills the primary keys.
func seedProduct(t *testing.T, db *gorm.DB, sku, name string, unitPrice int64) string {
	t.Helper()
	p := model.Product{
		SKU:       sku,
		Name:      name,
		Unit:      "tab",
		UnitPrice: unitPrice,
		Active:    true,
	}
	require.NoError(t, db.Create(&p).Error)
	base := model.ProductUnit{
		ProductID: p.ID,
		Name:      "tab",
		Factor:    1,
		IsBase:    true,
		SellPrice: unitPrice,
		Sellable:  true,
	}
	require.NoError(t, db.Create(&base).Error)
	return p.ID
}

// seedStock creates a batch for productID and posts an opening PURCHASE movement
// of qty base units into the MAIN warehouse, so CompleteSale's FEFO allocation
// can consume it. userID is the stamping user (FK to users.id).
func seedStock(t *testing.T, db *gorm.DB, productID, userID string, qty int32) string {
	t.Helper()
	b := model.Batch{
		ProductID:   productID,
		BatchNumber: "B-1",
		ExpiryDate:  time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC),
		CostPrice:   500,
		ReceivedAt:  time.Now(),
	}
	require.NoError(t, db.Create(&b).Error)
	mv := model.StockMovement{
		BatchID:     b.ID,
		Qty:         qty,
		Type:        "PURCHASE",
		Reason:      "opening",
		UserID:      userID,
		WarehouseID: mainWarehouseID,
	}
	require.NoError(t, db.Create(&mv).Error)
	return b.ID
}

// newSaleSvc is the common setup: fresh DB, bootstrap owner, sale service.
// Returns the service, an OWNER ctx, the gorm handle, and the owner id.
func newSaleSvc(t *testing.T) (*salesvc.SaleService, context.Context, *gorm.DB, string) {
	t.Helper()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := salesvc.NewSaleService(gormDB, cfg.Printer)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	return svc, ctx, gormDB, ownerID
}

// startDraft opens a DRAFT sale via the real RPC and returns its id.
func startDraft(t *testing.T, svc *salesvc.SaleService, ctx context.Context) string {
	t.Helper()
	resp, err := svc.StartSale(ctx, connect.NewRequest(&posifacev1.StartSaleRequest{}))
	require.NoError(t, err)
	return resp.Msg.Sale.Id
}
