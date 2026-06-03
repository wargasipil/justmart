package e2e

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
)

// TestListProducts_OpnameBeforeFilter pins the `opname_before` filter on
// ListProducts: products whose latest COMPLETED stocktake in the active
// warehouse is before the given date stay; products counted on/after the
// date drop out; never-counted products always stay (audit-overdue semantics).
func TestListProducts_OpnameBeforeFilter(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	cleanupLeftoverDrafts(t, env, ctx)

	uniq := time.Now().UnixNano()
	medA, batchIDsA := seedProductAndBatches(t, env, ctx,
		fmt.Sprintf("e2e-flt-A-%d", uniq), []int32{10})
	medB, _ := seedProductAndBatches(t, env, ctx,
		fmt.Sprintf("e2e-flt-B-%d", uniq), []int32{5})

	// Stocktake product A today (variance 0 → still counts as a touched line).
	start, err := env.Stocktakes.StartStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.StartStocktakeRequest{Name: "filter test"}))
	require.NoError(t, err)
	sessID := start.Msg.Session.Id
	t.Cleanup(func() {
		_, _ = env.Stocktakes.VoidStocktake(ctx, authReq(env, t,
			&stocktakeifacev1.VoidStocktakeRequest{SessionId: sessID}))
	})
	_, err = env.Stocktakes.AddBatchesToSession(ctx, authReq(env, t,
		&stocktakeifacev1.AddBatchesToSessionRequest{SessionId: sessID, BatchIds: batchIDsA}))
	require.NoError(t, err)
	get, err := env.Stocktakes.GetStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.GetStocktakeRequest{Id: sessID}))
	require.NoError(t, err)
	require.Len(t, get.Msg.Lines, 1)
	_, err = env.Stocktakes.RecordCount(ctx, authReq(env, t,
		&stocktakeifacev1.RecordCountRequest{LineId: get.Msg.Lines[0].Id, CountedQty: 10}))
	require.NoError(t, err)
	_, err = env.Stocktakes.CompleteStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.CompleteStocktakeRequest{SessionId: sessID}))
	require.NoError(t, err)

	// helper: list with opname_before + a query to scope to our seeded products.
	listIDs := func(before, query string) []string {
		res, err := env.Products.ListProducts(ctx, authReq(env, t,
			&inventoryifacev1.ListProductsRequest{
				OpnameBefore: before,
				Query:        query,
				Limit:        100,
			}))
		require.NoError(t, err)
		ids := make([]string, 0, len(res.Msg.Products))
		for _, m := range res.Msg.Products {
			ids = append(ids, m.Id)
		}
		return ids
	}

	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	// Both A (counted today, today < tomorrow) and B (never counted) appear.
	// Scope via the shared `e2e-flt-` prefix substring matches both.
	got := listIDs(tomorrow, fmt.Sprintf("e2e-flt-A-%d", uniq))
	require.Contains(t, got, medA, "A counted < tomorrow → kept")
	gotB := listIDs(tomorrow, fmt.Sprintf("e2e-flt-B-%d", uniq))
	require.Contains(t, gotB, medB, "B never counted → kept")

	// Filter "before yesterday": A was counted today (>= yesterday) → drops out; B never counted → stays.
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	gotA2 := listIDs(yesterday, fmt.Sprintf("e2e-flt-A-%d", uniq))
	require.NotContains(t, gotA2, medA, "A's last opname is today, not before yesterday → drops")
	gotB2 := listIDs(yesterday, fmt.Sprintf("e2e-flt-B-%d", uniq))
	require.Contains(t, gotB2, medB, "B never counted → stays under any opname_before")

	// Bogus date → InvalidArgument.
	_, err = env.Products.ListProducts(ctx, authReq(env, t,
		&inventoryifacev1.ListProductsRequest{OpnameBefore: "bogus", Limit: 10}))
	require.Error(t, err)
	var cerr *connect.Error
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeInvalidArgument, cerr.Code())
}

// TestListProducts_PopulatesLastStocktake covers the new enrichLastStocktake
// step: each page row carries the most recent COMPLETED stocktake date for
// the product in the active warehouse. Products never counted get "".
func TestListProducts_PopulatesLastStocktake(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	cleanupLeftoverDrafts(t, env, ctx)

	uniq := time.Now().UnixNano()
	prefix := fmt.Sprintf("e2e-lst-%d", uniq)
	medA, batchIDsA := seedProductAndBatches(t, env, ctx, prefix+"-A", []int32{10})
	medB, _ := seedProductAndBatches(t, env, ctx, prefix+"-B", []int32{5})

	// Stocktake A today.
	start, err := env.Stocktakes.StartStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.StartStocktakeRequest{Name: "enrich test"}))
	require.NoError(t, err)
	sessID := start.Msg.Session.Id
	t.Cleanup(func() {
		_, _ = env.Stocktakes.VoidStocktake(ctx, authReq(env, t,
			&stocktakeifacev1.VoidStocktakeRequest{SessionId: sessID}))
	})
	_, err = env.Stocktakes.AddBatchesToSession(ctx, authReq(env, t,
		&stocktakeifacev1.AddBatchesToSessionRequest{SessionId: sessID, BatchIds: batchIDsA}))
	require.NoError(t, err)
	get, err := env.Stocktakes.GetStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.GetStocktakeRequest{Id: sessID}))
	require.NoError(t, err)
	_, err = env.Stocktakes.RecordCount(ctx, authReq(env, t,
		&stocktakeifacev1.RecordCountRequest{LineId: get.Msg.Lines[0].Id, CountedQty: 10}))
	require.NoError(t, err)
	_, err = env.Stocktakes.CompleteStocktake(ctx, authReq(env, t,
		&stocktakeifacev1.CompleteStocktakeRequest{SessionId: sessID}))
	require.NoError(t, err)

	// List both products via a shared prefix query.
	res, err := env.Products.ListProducts(ctx, authReq(env, t,
		&inventoryifacev1.ListProductsRequest{Query: prefix, Limit: 100}))
	require.NoError(t, err)
	byID := map[string]*inventoryifacev1.Product{}
	for _, m := range res.Msg.Products {
		byID[m.Id] = m
	}
	today := time.Now().Format("2006-01-02")
	require.Contains(t, byID, medA)
	require.Equal(t, today, byID[medA].LastStocktakeDate, "A counted today → list row carries today's date")
	require.Contains(t, byID, medB)
	require.Equal(t, "", byID[medB].LastStocktakeDate, "B never counted → list row carries empty")
}
