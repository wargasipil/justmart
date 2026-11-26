package product_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	productsvc "github.com/justmart/backend/internal/service/product"
	"github.com/justmart/backend/internal/service/servicetest"
)

// GetProduct's "Last cost" carries the latest lot's exact pack cost beside its
// per-base cost_price, read from that SAME lot, so the page can show "Rp 65.000
// / box" for a box bought at 65.000 rather than 2.167 x 30 = 65.010.
func TestGetProduct_ReferenceCostPackFromLatestBatch(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	pid := seedProduct(t, svc, ctx, "SKU-REFPACK", "Ref pack product", 3000)
	now := time.Now()
	// Older lot priced per box of 10; the newest one per box of 30. Only the
	// newest may surface.
	older := model.Batch{ProductID: pid, BatchNumber: "OLD", ExpiryDate: now.AddDate(1, 0, 0),
		CostPrice: 2_000, CostPackPrice: 20_000, CostPackFactor: 10, ReceivedAt: now.AddDate(0, 0, -10)}
	require.NoError(t, gormDB.Create(&older).Error)
	latest := model.Batch{ProductID: pid, BatchNumber: "NEW", ExpiryDate: now.AddDate(1, 0, 0),
		CostPrice: 2_167, CostPackPrice: 65_000, CostPackFactor: 30, ReceivedAt: now.AddDate(0, 0, -1)}
	require.NoError(t, gormDB.Create(&latest).Error)

	resp, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: pid}))
	require.NoError(t, err)
	p := resp.Msg.Product
	require.Equal(t, int64(2_167), p.ReferenceCost)
	require.Equal(t, int64(65_000), p.ReferenceCostPack)
	require.Equal(t, int64(30), p.ReferenceCostPackFactor)
}

// A newest lot that records no pack (manual / imported / pre-00064 stock)
// reports 0/0 -- even when an OLDER lot had one -- so the client derives from
// reference_cost instead of pairing two different lots' figures.
func TestGetProduct_ReferenceCostPackZeroWhenLatestHasNone(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	pid := seedProduct(t, svc, ctx, "SKU-REFPACK-0", "Ref pack legacy", 3000)
	now := time.Now()
	older := model.Batch{ProductID: pid, BatchNumber: "OLD", ExpiryDate: now.AddDate(1, 0, 0),
		CostPrice: 2_167, CostPackPrice: 65_000, CostPackFactor: 30, ReceivedAt: now.AddDate(0, 0, -10)}
	require.NoError(t, gormDB.Create(&older).Error)
	latest := model.Batch{ProductID: pid, BatchNumber: "NEW", ExpiryDate: now.AddDate(1, 0, 0),
		CostPrice: 2_100, ReceivedAt: now.AddDate(0, 0, -1)}
	require.NoError(t, gormDB.Create(&latest).Error)

	resp, err := svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: pid}))
	require.NoError(t, err)
	p := resp.Msg.Product
	require.Equal(t, int64(2_100), p.ReferenceCost)
	require.Zero(t, p.ReferenceCostPack)
	require.Zero(t, p.ReferenceCostPackFactor)

	// And a product never received at all has nothing to report.
	none := seedProduct(t, svc, ctx, "SKU-REFPACK-NONE", "Never received", 3000)
	resp, err = svc.GetProduct(ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: none}))
	require.NoError(t, err)
	require.Zero(t, resp.Msg.Product.ReferenceCost)
	require.Zero(t, resp.Msg.Product.ReferenceCostPack)
	require.Zero(t, resp.Msg.Product.ReferenceCostPackFactor)
}
