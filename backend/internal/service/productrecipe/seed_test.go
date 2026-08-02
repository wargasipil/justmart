package productrecipe_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
	productrecipesvc "github.com/justmart/backend/internal/service/productrecipe"
	"github.com/justmart/backend/internal/service/servicetest"
)

// mainWarehouseID is the migration-seeded default warehouse. An OWNER with no
// explicit membership resolves here, so component stock is seeded here to match
// what ListProductRecipeItems reads.
const mainWarehouseID = "00000000-0000-0000-0000-0000000000a1"

// newRecipeEnv is the common setup: fresh DB, bootstrap owner, recipe service,
// and an OWNER context (the List RPC resolves the caller's active warehouse, so
// a principal is required).
func newRecipeEnv(t *testing.T) (*productrecipesvc.ProductRecipeService, context.Context, *gorm.DB, string) {
	t.Helper()
	db, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, db, cfg)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	return productrecipesvc.NewProductRecipeService(db), ctx, db, ownerID
}

// seedProductOfKind inserts a product of the given kind. Components must be
// STOCKED; the parent of a recipe must be COMPOSITE.
func seedProductOfKind(t *testing.T, db *gorm.DB, sku, kind string) string {
	t.Helper()
	p := model.Product{
		SKU:       sku,
		Name:      sku + " Item",
		Unit:      "g",
		UnitPrice: 1000,
		Active:    true,
		Kind:      kind,
	}
	require.NoError(t, db.Create(&p).Error)
	return p.ID
}

// seedComponentStock posts an opening PURCHASE movement of qty base units for a
// component into MAIN, so the recipe card's component_ready / buildable have
// something to report.
func seedComponentStock(t *testing.T, db *gorm.DB, productID, userID string, qty int32) {
	t.Helper()
	b := model.Batch{
		ProductID:   productID,
		BatchNumber: "B-" + productID[:8],
		ExpiryDate:  time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC),
		CostPrice:   100,
		ReceivedAt:  time.Now(),
	}
	require.NoError(t, db.Create(&b).Error)
	require.NoError(t, db.Create(&model.StockMovement{
		BatchID:     b.ID,
		Qty:         qty,
		Type:        "PURCHASE",
		Reason:      "opening",
		UserID:      userID,
		WarehouseID: mainWarehouseID,
	}).Error)
}

func addLine(
	t *testing.T,
	svc *productrecipesvc.ProductRecipeService,
	ctx context.Context,
	parentID, componentID string,
	qtyBase int64,
) *inventoryifacev1.ProductRecipeItem {
	t.Helper()
	r, err := svc.CreateProductRecipeItem(ctx, connect.NewRequest(&inventoryifacev1.CreateProductRecipeItemRequest{
		ProductId:          parentID,
		ComponentProductId: componentID,
		QtyBase:            qtyBase,
	}))
	require.NoError(t, err)
	return r.Msg.Item
}

func requireToken(t *testing.T, err error, code connect.Code, token string) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, code, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, token, ce.Message())
}

// compositeKind / stockedKind / serviceKind alias the shared constants so the
// tests read as intent rather than string literals.
const (
	compositeKind = common.ProductKindComposite
	stockedKind   = common.ProductKindStocked
	serviceKind   = common.ProductKindService
)
