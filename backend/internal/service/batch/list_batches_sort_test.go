package batch_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	batchsvc "github.com/justmart/backend/internal/service/batch"
	"github.com/justmart/backend/internal/service/servicetest"
)

// dateIn is n days from today as YYYY-MM-DD. Expiries are relative so the
// lots stay in the future whatever day the suite runs (CLAUDE.md: a test date
// that means "now" is computed, never a literal).
func dateIn(n int) string { return time.Now().AddDate(0, 0, n).Format("2006-01-02") }

// Three in-stock lots whose batch number, expiry, cost, quantity and supplier
// each rank them in a DIFFERENT order, so every sort below has one answer that
// no other column could produce by accident:
//
//	lot     expiry  qty  cost  supplier
//	SRT-A   +300d    30  2000  Bumi Farma
//	SRT-B   +100d    10  1000  Cahaya Medika
//	SRT-C   +200d    20  3000  Anugrah Sehat
//
// plus an emptied lot that only_in_stock must keep out of every order (the
// tab's real request: the qty sort runs alongside the HAVING clause).
func TestListBatches_Sort(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "SRT-SKU-1", "Sort Med")
	supplier := func(code, name string) string {
		t.Helper()
		s := model.Supplier{Code: code, Name: name, Active: true}
		require.NoError(t, gormDB.Create(&s).Error)
		return s.ID
	}
	bumi := supplier("SRT-SUP-1", "Bumi Farma")
	cahaya := supplier("SRT-SUP-2", "Cahaya Medika")
	anugrah := supplier("SRT-SUP-3", "Anugrah Sehat")

	lot := func(no, expiry string, cost, qty int64, supplierID string) {
		t.Helper()
		_, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
			ProductId:       prodID,
			SupplierId:      supplierID,
			BatchNumber:     no,
			ExpiryDate:      expiry,
			CostPrice:       cost,
			InitialQuantity: qty,
		}))
		require.NoError(t, err)
	}
	lot("SRT-A", dateIn(300), 2000, 30, bumi)
	lot("SRT-B", dateIn(100), 1000, 10, cahaya)
	lot("SRT-C", dateIn(200), 3000, 20, anugrah)
	lot("SRT-EMPTY", dateIn(50), 500, 0, bumi)

	list := func(sortBy string, desc bool) []string {
		t.Helper()
		resp, err := svc.ListBatches(ctx, connect.NewRequest(&inventoryifacev1.ListBatchesRequest{
			ProductId:   prodID,
			OnlyInStock: true,
			SortBy:      sortBy,
			SortDesc:    desc,
		}))
		require.NoError(t, err)
		require.Equal(t, int32(3), resp.Msg.Total, "sorting must not change the count")
		nos := make([]string, 0, len(resp.Msg.Batches))
		for _, b := range resp.Msg.Batches {
			nos = append(nos, b.BatchNumber)
		}
		return nos
	}

	require.Equal(t, []string{"SRT-B", "SRT-C", "SRT-A"}, list("", false), "default = soonest expiry first")
	require.Equal(t, []string{"SRT-B", "SRT-C", "SRT-A"}, list("", true), "sort_desc is ignored without sort_by")
	require.Equal(t, []string{"SRT-B", "SRT-C", "SRT-A"}, list("expiry", false))
	require.Equal(t, []string{"SRT-A", "SRT-C", "SRT-B"}, list("expiry", true))
	require.Equal(t, []string{"SRT-A", "SRT-C", "SRT-B"}, list("qty", true))
	require.Equal(t, []string{"SRT-B", "SRT-A", "SRT-C"}, list("cost", false))
	require.Equal(t, []string{"SRT-C", "SRT-B", "SRT-A"}, list("batch_number", true))
	require.Equal(t, []string{"SRT-C", "SRT-A", "SRT-B"}, list("supplier", false))
}

// A sorted list still pages: one row per page walks the same order the full
// list reports, with nothing skipped or repeated across the page boundaries.
func TestListBatches_SortPagesStably(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	prodID := seedProduct(t, gormDB, "SRP-SKU-1", "Sort Page Med")
	// Equal cost on every lot: only the b.id tiebreaker decides their order.
	for _, no := range []string{"SRP-1", "SRP-2", "SRP-3"} {
		_, err := svc.CreateBatch(ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
			ProductId:       prodID,
			BatchNumber:     no,
			ExpiryDate:      dateIn(120),
			CostPrice:       1500,
			InitialQuantity: 5,
		}))
		require.NoError(t, err)
	}

	page := func(limit, offset int32) []string {
		t.Helper()
		resp, err := svc.ListBatches(ctx, connect.NewRequest(&inventoryifacev1.ListBatchesRequest{
			ProductId: prodID,
			SortBy:    "cost",
			SortDesc:  true,
			Limit:     limit,
			Offset:    offset,
		}))
		require.NoError(t, err)
		require.Equal(t, int32(3), resp.Msg.Total)
		out := make([]string, 0, len(resp.Msg.Batches))
		for _, b := range resp.Msg.Batches {
			out = append(out, b.Id)
		}
		return out
	}

	all := page(3, 0)
	require.Len(t, all, 3)
	var walked []string
	for off := int32(0); off < 3; off++ {
		walked = append(walked, page(1, off)...)
	}
	require.Equal(t, all, walked, "ties must break the same way on every page")
}

// sort_by is a whitelist: anything else is refused with a stable token rather
// than reaching the ORDER BY.
func TestListBatches_SortInvalid(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := batchsvc.NewBatchService(gormDB)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	for _, sortBy := range []string{"nope", "b.id; DROP TABLE batches", "qty DESC"} {
		_, err := svc.ListBatches(ctx, connect.NewRequest(&inventoryifacev1.ListBatchesRequest{
			SortBy: sortBy,
		}))
		requireBatchToken(t, err, connect.CodeInvalidArgument, "batch.sort_invalid")
	}
}
