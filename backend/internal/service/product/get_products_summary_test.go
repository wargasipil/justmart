package product_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	productsvc "github.com/justmart/backend/internal/service/product"
	"github.com/justmart/backend/internal/service/servicetest"
)

// TestGetProductsSummary_AggregatesReadyAndOnOrder proves the stat row sums
// BOTH counts and BOTH valuations across every matching product — not one page,
// not one product.
func TestGetProductsSummary_AggregatesReadyAndOnOrder(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	mainWH := defaultWarehouseID(t, gormDB)
	sup := seedSupplierRow(t, gormDB, "SUM-A", "Summary supplier")
	p1 := seedProduct(t, svc, ctx, "sum-1", "Summary one", 5_000)
	p2 := seedProduct(t, svc, ctx, "sum-2", "Summary two", 7_000)

	// Ready: 10 @ 1_000 + 4 @ 2_500 = 14 units, 20_000 at cost.
	seedBatchWithStock(t, gormDB, p1, mainWH, ownerID, 10, 1_000)
	seedBatchWithStock(t, gormDB, p2, mainWH, ownerID, 4, 2_500)

	// On-order: p1 has 30 ordered / 12 received = 18 outstanding at a NET rate
	// of 9_000/30 = 300 (discounted off the 400 gross, so this also pins that
	// the valuation uses netUnitCost rather than the gross column).
	seedOpenPOItem(t, gormDB, p1, sup, mainWH, ownerID, 30, 12, 400, 9_000)
	// p2 has 5 ordered / 5 received = nothing outstanding; it must not count.
	seedOpenPOItem(t, gormDB, p2, sup, mainWH, ownerID, 5, 5, 1_000, 5_000)

	resp, err := svc.GetProductsSummary(ctx,
		connect.NewRequest(&inventoryifacev1.GetProductsSummaryRequest{}))
	require.NoError(t, err)
	require.Equal(t, int64(14), resp.Msg.ReadyStock)
	require.Equal(t, int64(10*1_000+4*2_500), resp.Msg.ReadyValuation)
	require.Equal(t, int64(18), resp.Msg.OnOrderStock)
	require.Equal(t, int64(18*300), resp.Msg.OnOrderValuation)
}

// TestGetProductsSummary_HonorsListFilters is the load-bearing one: the stat row
// must describe exactly the set the table under it shows. Each filter is checked
// against the ListProducts result for the same request.
func TestGetProductsSummary_HonorsListFilters(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	mainWH := defaultWarehouseID(t, gormDB)
	keep := seedProduct(t, svc, ctx, "flt-keep", "Keepme tablet", 1_000)
	other := seedProduct(t, svc, ctx, "flt-other", "Otherthing tablet", 1_000)
	seedBatchWithStock(t, gormDB, keep, mainWH, ownerID, 6, 500)
	seedBatchWithStock(t, gormDB, other, mainWH, ownerID, 9, 700)

	// Query filter: only the matching product's stock is summed.
	sum, err := svc.GetProductsSummary(ctx,
		connect.NewRequest(&inventoryifacev1.GetProductsSummaryRequest{Query: "Keepme"}))
	require.NoError(t, err)
	require.Equal(t, int64(6), sum.Msg.ReadyStock)
	require.Equal(t, int64(6*500), sum.Msg.ReadyValuation)
	list, err := svc.ListProducts(ctx,
		connect.NewRequest(&inventoryifacev1.ListProductsRequest{Query: "Keepme"}))
	require.NoError(t, err)
	require.Equal(t, int32(1), list.Msg.Total, "summary and list must describe the same set")

	// Archived tab: an archived product leaves the default (active-only) summary
	// and lands in the only_archived one.
	_, err = svc.ArchiveProduct(ctx,
		connect.NewRequest(&inventoryifacev1.ArchiveProductRequest{Id: other}))
	require.NoError(t, err)

	sum, err = svc.GetProductsSummary(ctx,
		connect.NewRequest(&inventoryifacev1.GetProductsSummaryRequest{}))
	require.NoError(t, err)
	require.Equal(t, int64(6), sum.Msg.ReadyStock, "archived stock must drop out of the active summary")

	sum, err = svc.GetProductsSummary(ctx,
		connect.NewRequest(&inventoryifacev1.GetProductsSummaryRequest{OnlyArchived: true}))
	require.NoError(t, err)
	require.Equal(t, int64(9), sum.Msg.ReadyStock)
	require.Equal(t, int64(9*700), sum.Msg.ReadyValuation)

	// include_inactive spans both.
	sum, err = svc.GetProductsSummary(ctx,
		connect.NewRequest(&inventoryifacev1.GetProductsSummaryRequest{IncludeInactive: true}))
	require.NoError(t, err)
	require.Equal(t, int64(15), sum.Msg.ReadyStock)
}

// TestGetProductsSummary_ReadyIsWarehouseScoped pins the split scoping: ready
// follows the caller's active warehouse, on-order stays company-wide (POs carry
// no warehouse — the documented carve-out).
func TestGetProductsSummary_ReadyIsWarehouseScoped(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	mainWH := defaultWarehouseID(t, gormDB)
	sideWH := seedWarehouseRow(t, gormDB, "SUM-WH2", "Second warehouse")
	sup := seedSupplierRow(t, gormDB, "SUM-B", "Scoping supplier")
	prod := seedProduct(t, svc, ctx, "sum-wh", "Scoped product", 1_000)

	seedBatchWithStock(t, gormDB, prod, mainWH, ownerID, 7, 100)
	seedBatchWithStock(t, gormDB, prod, sideWH, ownerID, 50, 100)
	seedOpenPOItem(t, gormDB, prod, sup, sideWH, ownerID, 20, 0, 300, 6_000)

	// Default (MAIN) sees only MAIN's on-hand...
	sum, err := svc.GetProductsSummary(ctx,
		connect.NewRequest(&inventoryifacev1.GetProductsSummaryRequest{}))
	require.NoError(t, err)
	require.Equal(t, int64(7), sum.Msg.ReadyStock)
	require.Equal(t, int64(700), sum.Msg.ReadyValuation)
	// ...but the whole company's outstanding POs, from either warehouse's caller.
	require.Equal(t, int64(20), sum.Msg.OnOrderStock)
	require.Equal(t, int64(20*300), sum.Msg.OnOrderValuation)

	sideCtx := servicetest.CtxInWarehouse(context.Background(),
		"OWNER", ownerID, sideWH)
	sum, err = svc.GetProductsSummary(sideCtx,
		connect.NewRequest(&inventoryifacev1.GetProductsSummaryRequest{}))
	require.NoError(t, err)
	require.Equal(t, int64(50), sum.Msg.ReadyStock)
	require.Equal(t, int64(5_000), sum.Msg.ReadyValuation)
	require.Equal(t, int64(20), sum.Msg.OnOrderStock)
}

// TestGetProductsSummary_EmptyAndInvalid covers the two edges: a filter matching
// nothing returns zeros (not an error, and no empty IN () reaching the driver),
// and a malformed opname_before is rejected the same way ListProducts rejects it.
func TestGetProductsSummary_EmptyAndInvalid(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	seedProduct(t, svc, ctx, "sum-edge", "Edge product", 1_000)

	sum, err := svc.GetProductsSummary(ctx,
		connect.NewRequest(&inventoryifacev1.GetProductsSummaryRequest{Query: "no-such-product"}))
	require.NoError(t, err)
	require.Equal(t, int64(0), sum.Msg.ReadyStock)
	require.Equal(t, int64(0), sum.Msg.ReadyValuation)
	require.Equal(t, int64(0), sum.Msg.OnOrderStock)
	require.Equal(t, int64(0), sum.Msg.OnOrderValuation)

	_, err = svc.GetProductsSummary(ctx,
		connect.NewRequest(&inventoryifacev1.GetProductsSummaryRequest{OpnameBefore: "31-12-2026"}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
