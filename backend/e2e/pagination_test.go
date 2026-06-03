package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

// TestListProducts_Pagination seeds more rows than a page holds and asserts
// limit/offset/total behave: page 1 and page 2 are disjoint and total counts
// every matching row regardless of the page window.
func TestListProducts_Pagination(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()
	// A shared name token scopes the query to just these rows.
	token := fmt.Sprintf("PAGE-%d", uniq)

	const seeded = 5
	for i := 0; i < seeded; i++ {
		med, err := env.Products.CreateProduct(ctx, authReq(env, t,
			&inventoryifacev1.CreateProductRequest{
				Sku:       fmt.Sprintf("pg-%d-%d", uniq, i),
				Name:      fmt.Sprintf("%s item %02d", token, i),
				Unit:      "tab",
				UnitPrice: 1000,
			}))
		require.NoError(t, err)
		id := med.Msg.Product.Id
		t.Cleanup(func() {
			_, _ = env.Products.ArchiveProduct(ctx, authReq(env, t,
				&inventoryifacev1.ArchiveProductRequest{Id: id}))
		})
	}

	// Page 1: limit 2, offset 0.
	p1, err := env.Products.ListProducts(ctx, authReq(env, t,
		&inventoryifacev1.ListProductsRequest{Query: token, Limit: 2, Offset: 0}))
	require.NoError(t, err)
	require.Len(t, p1.Msg.Products, 2)
	require.Equal(t, int32(seeded), p1.Msg.Total, "total counts all matching rows")

	// Page 2: limit 2, offset 2 — disjoint from page 1.
	p2, err := env.Products.ListProducts(ctx, authReq(env, t,
		&inventoryifacev1.ListProductsRequest{Query: token, Limit: 2, Offset: 2}))
	require.NoError(t, err)
	require.Len(t, p2.Msg.Products, 2)
	require.Equal(t, int32(seeded), p2.Msg.Total)
	require.NotEqual(t, p1.Msg.Products[0].Id, p2.Msg.Products[0].Id, "pages must differ")
	require.NotEqual(t, p1.Msg.Products[1].Id, p2.Msg.Products[0].Id)

	// Page 3: the remainder.
	p3, err := env.Products.ListProducts(ctx, authReq(env, t,
		&inventoryifacev1.ListProductsRequest{Query: token, Limit: 2, Offset: 4}))
	require.NoError(t, err)
	require.Len(t, p3.Msg.Products, 1)
	require.Equal(t, int32(seeded), p3.Msg.Total)
}

// TestListProducts_ReadyAndOnOrder proves the enriched stock columns: ready
// reflects on-hand in the active warehouse, on_order reflects open-PO quantity.
func TestListProducts_ReadyAndOnOrder(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("STK%d", uniq%100000))

	medName := fmt.Sprintf("Stock-Med-%d", uniq)
	med, err := env.Products.CreateProduct(ctx, authReq(env, t,
		&inventoryifacev1.CreateProductRequest{
			Sku: fmt.Sprintf("stk-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Product.Id
	t.Cleanup(func() {
		_, _ = env.Products.ArchiveProduct(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveProductRequest{Id: medID}))
	})

	// Seed 8 on-hand into this warehouse.
	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			ProductId: medID, BatchNumber: "STK-B1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 8,
		}, wh))
	require.NoError(t, err)

	// Create + send a PO for 5 more (open → counts as on-order).
	sup, err := env.Suppliers.CreateSupplier(ctx, authReq(env, t,
		&inventoryifacev1.CreateSupplierRequest{
			Name: "Stock sup", Code: fmt.Sprintf("STKSUP%d", uniq%100000),
		}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.Suppliers.ArchiveSupplier(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveSupplierRequest{Id: sup.Msg.Supplier.Id}))
	})
	po, err := env.POs.CreatePurchaseOrder(ctx, authReq(env, t,
		&purchasingifacev1.CreatePurchaseOrderRequest{
			SupplierId: sup.Msg.Supplier.Id,
			Items: []*purchasingifacev1.PurchaseOrderItemInput{
				{ProductId: medID, OrderedQty: 5, UnitCostPrice: 500},
			},
		}))
	require.NoError(t, err)
	_, err = env.POs.SendPurchaseOrder(ctx, authReq(env, t,
		&purchasingifacev1.SendPurchaseOrderRequest{Id: po.Msg.Order.Id}))
	require.NoError(t, err)

	// List from this warehouse: ready = 8, on_order = 5.
	res, err := env.Products.ListProducts(ctx, whReq(env, t,
		&inventoryifacev1.ListProductsRequest{Query: medName}, wh))
	require.NoError(t, err)
	var got *inventoryifacev1.Product
	for _, m := range res.Msg.Products {
		if m.Id == medID {
			got = m
		}
	}
	require.NotNil(t, got, "seeded product should be in the list")
	require.Equal(t, int64(8), got.ReadyStock, "ready = on-hand in active warehouse")
	require.Equal(t, int64(5), got.OnOrderStock, "on_order = open PO outstanding qty")
}

// TestListSales_SearchAndDateRange proves a completed sale is findable by the
// product sold and by a created-date range.
func TestListSales_SearchAndDateRange(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("SAL%d", uniq%100000))

	medName := fmt.Sprintf("Sold-Med-%d", uniq)
	med, err := env.Products.CreateProduct(ctx, authReq(env, t,
		&inventoryifacev1.CreateProductRequest{
			Sku: fmt.Sprintf("sal-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Product.Id
	t.Cleanup(func() {
		_, _ = env.Products.ArchiveProduct(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveProductRequest{Id: medID}))
	})

	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			ProductId: medID, BatchNumber: "SAL-B1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 10,
		}, wh))
	require.NoError(t, err)

	saleID := startSaleWith(env, t, ctx, wh)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: saleID, ProductId: medID, Qty: 2}, wh))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 2000,
		}, wh))
	require.NoError(t, err)

	// Search by the product name finds the sale and denormalizes the item name.
	// ListSales is warehouse-scoped, so pass the seeded warehouse's header.
	res, err := env.Sales.ListSales(ctx, whReq(env, t,
		&posifacev1.ListSalesRequest{Query: medName}, wh))
	require.NoError(t, err)
	got := findSale(res.Msg.Sales, saleID)
	require.NotNil(t, got, "search by product name should return the sale")
	require.GreaterOrEqual(t, res.Msg.Total, int32(1))
	foundItem := false
	for _, it := range got.Items {
		if it.ProductName == medName {
			foundItem = true
		}
	}
	require.True(t, foundItem, "sale item should carry the denormalized product name")

	// Created-date range includes it.
	from := time.Now().AddDate(0, 0, -1).Unix()
	to := time.Now().AddDate(0, 0, 2).Unix()
	res, err = env.Sales.ListSales(ctx, whReq(env, t,
		&posifacev1.ListSalesRequest{Query: medName, FromUnix: from, ToUnix: to}, wh))
	require.NoError(t, err)
	require.NotNil(t, findSale(res.Msg.Sales, saleID), "date range should include the sale")
}

// TestListSales_ExcludesDraft proves order history shows finalized orders only:
// an abandoned DRAFT cart never appears under "All" (UNSPECIFIED) status, while
// a completed sale does.
func TestListSales_ExcludesDraft(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("DRF%d", uniq%100000))

	medName := fmt.Sprintf("Draft-Med-%d", uniq)
	med, err := env.Products.CreateProduct(ctx, authReq(env, t,
		&inventoryifacev1.CreateProductRequest{
			Sku: fmt.Sprintf("drf-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Product.Id
	t.Cleanup(func() {
		_, _ = env.Products.ArchiveProduct(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveProductRequest{Id: medID}))
	})

	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			ProductId: medID, BatchNumber: "DRF-B1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 20,
		}, wh))
	require.NoError(t, err)

	// A completed sale.
	doneID := startSaleWith(env, t, ctx, wh)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: doneID, ProductId: medID, Qty: 2}, wh))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: doneID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 2000,
		}, wh))
	require.NoError(t, err)

	// An abandoned DRAFT sale (started + item added, never completed).
	draftID := startSaleWith(env, t, ctx, wh)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: draftID, ProductId: medID, Qty: 1}, wh))
	require.NoError(t, err)

	// "All" (UNSPECIFIED): completed in, draft out. Both reference medName, so the
	// search would surface the draft too were it not for the status<>DRAFT filter.
	// ListSales is warehouse-scoped, so pass the seeded warehouse's header.
	res, err := env.Sales.ListSales(ctx, whReq(env, t,
		&posifacev1.ListSalesRequest{Query: medName}, wh))
	require.NoError(t, err)
	require.NotNil(t, findSale(res.Msg.Sales, doneID), "completed sale appears in history")
	require.Nil(t, findSale(res.Msg.Sales, draftID), "draft sale is excluded from history")

	// Explicit COMPLETED filter still excludes the draft.
	res, err = env.Sales.ListSales(ctx, whReq(env, t,
		&posifacev1.ListSalesRequest{
			Query: medName, Status: posifacev1.SaleStatus_SALE_STATUS_COMPLETED,
		}, wh))
	require.NoError(t, err)
	require.NotNil(t, findSale(res.Msg.Sales, doneID))
	require.Nil(t, findSale(res.Msg.Sales, draftID))
}

func findSale(rows []*posifacev1.Sale, id string) *posifacev1.Sale {
	for _, r := range rows {
		if r.Id == id {
			return r
		}
	}
	return nil
}

// TestGetSalesSummary proves the order-history summary aggregates over ALL
// matching rows (server-side), honoring the same status/date/search filters as
// ListSales — not a client-side sum of a page.
func TestGetSalesSummary(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("SUM%d", uniq%100000))

	medName := fmt.Sprintf("Summary-Med-%d", uniq)
	med, err := env.Products.CreateProduct(ctx, authReq(env, t,
		&inventoryifacev1.CreateProductRequest{
			Sku: fmt.Sprintf("sum-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Product.Id
	t.Cleanup(func() {
		_, _ = env.Products.ArchiveProduct(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveProductRequest{Id: medID}))
	})

	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			ProductId: medID, BatchNumber: "SUM-B1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 10,
		}, wh))
	require.NoError(t, err)

	saleID := startSaleWith(env, t, ctx, wh)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: saleID, ProductId: medID, Qty: 3}, wh))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 3000,
		}, wh))
	require.NoError(t, err)

	// Scope the summary to this product (query=medName) so the figures are exact:
	// 1 sale, 3 units sold, Rp 3000 revenue.
	sum, err := env.Sales.GetSalesSummary(ctx, whReq(env, t,
		&posifacev1.GetSalesSummaryRequest{
			Query:  medName,
			Status: posifacev1.SaleStatus_SALE_STATUS_COMPLETED,
		}, wh))
	require.NoError(t, err)
	require.Equal(t, int64(1), sum.Msg.SaleCount, "exactly one matching sale")
	require.Equal(t, int64(3), sum.Msg.ItemsSold, "sum of qty")
	require.Equal(t, int64(3000), sum.Msg.Revenue, "sum of total")

	// A date window that ends before the sale was created → all zeros.
	past := time.Now().AddDate(0, 0, -10).Unix()
	pastEnd := time.Now().AddDate(0, 0, -5).Unix()
	empty, err := env.Sales.GetSalesSummary(ctx, whReq(env, t,
		&posifacev1.GetSalesSummaryRequest{
			Query:    medName,
			Status:   posifacev1.SaleStatus_SALE_STATUS_COMPLETED,
			FromUnix: past,
			ToUnix:   pastEnd,
		}, wh))
	require.NoError(t, err)
	require.Equal(t, int64(0), empty.Msg.SaleCount)
	require.Equal(t, int64(0), empty.Msg.ItemsSold)
	require.Equal(t, int64(0), empty.Msg.Revenue)
}

// TestListSales_WarehouseScoped proves ListSales (+ GetSalesSummary) filter
// the order-history by the caller's active warehouse: a sale completed in
// WH-A is invisible from WH-B.
func TestListSales_WarehouseScoped(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	whA := makeWarehouse(env, t, ctx, fmt.Sprintf("LSWA%d", uniq%100000))
	whB := makeWarehouse(env, t, ctx, fmt.Sprintf("LSWB%d", uniq%100000))

	medName := fmt.Sprintf("Scope-Med-%d", uniq)
	med, err := env.Products.CreateProduct(ctx, authReq(env, t,
		&inventoryifacev1.CreateProductRequest{
			Sku: fmt.Sprintf("scp-%d", uniq), Name: medName, Unit: "tab", UnitPrice: 1000,
		}))
	require.NoError(t, err)
	medID := med.Msg.Product.Id
	t.Cleanup(func() {
		_, _ = env.Products.ArchiveProduct(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveProductRequest{Id: medID}))
	})

	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			ProductId: medID, BatchNumber: "SCP-B1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 5,
		}, whA))
	require.NoError(t, err)

	// Complete a sale in WH-A.
	saleID := startSaleWith(env, t, ctx, whA)
	_, err = env.Sales.AddItem(ctx, whReq(env, t,
		&posifacev1.AddItemRequest{SaleId: saleID, ProductId: medID, Qty: 2}, whA))
	require.NoError(t, err)
	_, err = env.Sales.CompleteSale(ctx, whReq(env, t,
		&posifacev1.CompleteSaleRequest{
			SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 2000,
		}, whA))
	require.NoError(t, err)

	// WH-A sees the sale.
	resA, err := env.Sales.ListSales(ctx, whReq(env, t,
		&posifacev1.ListSalesRequest{Query: medName}, whA))
	require.NoError(t, err)
	require.NotNil(t, findSale(resA.Msg.Sales, saleID), "WH-A finds the sale")

	// WH-B does not.
	resB, err := env.Sales.ListSales(ctx, whReq(env, t,
		&posifacev1.ListSalesRequest{Query: medName}, whB))
	require.NoError(t, err)
	require.Nil(t, findSale(resB.Msg.Sales, saleID), "WH-B does not find the sale")

	// GetSalesSummary mirrors the scope.
	sumA, err := env.Sales.GetSalesSummary(ctx, whReq(env, t,
		&posifacev1.GetSalesSummaryRequest{Query: medName,
			Status: posifacev1.SaleStatus_SALE_STATUS_COMPLETED}, whA))
	require.NoError(t, err)
	require.GreaterOrEqual(t, sumA.Msg.SaleCount, int64(1), "WH-A summary includes the sale")

	sumB, err := env.Sales.GetSalesSummary(ctx, whReq(env, t,
		&posifacev1.GetSalesSummaryRequest{Query: medName,
			Status: posifacev1.SaleStatus_SALE_STATUS_COMPLETED}, whB))
	require.NoError(t, err)
	require.Equal(t, int64(0), sumB.Msg.SaleCount, "WH-B summary excludes the sale")
}

// TestGetProduct_EnrichAndMovementsByProduct proves the detail page's two new
// backend bits: GetProduct fills ready_stock (active warehouse), and
// ListMovements{product_id} returns that product's movements (and excludes
// another product's).
func TestGetProduct_EnrichAndMovementsByProduct(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	wh := makeWarehouse(env, t, ctx, fmt.Sprintf("MED%d", uniq%100000))

	mk := func(suffix string) string {
		m, err := env.Products.CreateProduct(ctx, authReq(env, t,
			&inventoryifacev1.CreateProductRequest{
				Sku:  fmt.Sprintf("md-%d-%s", uniq, suffix),
				Name: fmt.Sprintf("Detail-Med-%d-%s", uniq, suffix),
				Unit: "tab", UnitPrice: 1000,
			}))
		require.NoError(t, err)
		id := m.Msg.Product.Id
		t.Cleanup(func() {
			_, _ = env.Products.ArchiveProduct(ctx, authReq(env, t,
				&inventoryifacev1.ArchiveProductRequest{Id: id}))
		})
		return id
	}
	medA := mk("a")
	medB := mk("b")

	// A supplier so the batch (and thus last_restock_supplier) has one.
	sup, err := env.Suppliers.CreateSupplier(ctx, authReq(env, t,
		&inventoryifacev1.CreateSupplierRequest{
			Name: "Restock Sup", Code: fmt.Sprintf("RST%d", uniq%100000),
		}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.Suppliers.ArchiveSupplier(ctx, authReq(env, t,
			&inventoryifacev1.ArchiveSupplierRequest{Id: sup.Msg.Supplier.Id}))
	})

	// Seed 7 of medA into this warehouse (creates a PURCHASE movement of +7),
	// received on a known date from the supplier.
	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			ProductId: medA, BatchNumber: "MD-A1", ExpiryDate: "2099-12-31",
			CostPrice: 500, InitialQuantity: 7,
			SupplierId: sup.Msg.Supplier.Id, ReceivedAt: "2026-05-20",
		}, wh))
	require.NoError(t, err)

	// GetProduct from this warehouse → ready_stock + last-restock enriched.
	got, err := env.Products.GetProduct(ctx, whReq(env, t,
		&inventoryifacev1.GetProductRequest{Id: medA}, wh))
	require.NoError(t, err)
	require.Equal(t, int64(7), got.Msg.Product.ReadyStock, "GetProduct fills ready_stock")
	require.Equal(t, "2026-05-20", got.Msg.Product.LastRestockDate, "last restock date")
	require.Equal(t, "Restock Sup", got.Msg.Product.LastRestockSupplier, "last restock supplier")
	require.Equal(t, int64(7), got.Msg.Product.TotalStock, "total stock (all warehouses)")
	require.Equal(t, int64(3500), got.Msg.Product.StockValuation, "valuation = 7 × 500")
	require.Equal(t, int64(500), got.Msg.Product.ReferenceCost, "reference cost = the only batch's cost")

	// A newer batch at a higher cost → reference_cost follows the latest received.
	_, err = env.Batches.CreateBatch(ctx, whReq(env, t,
		&inventoryifacev1.CreateBatchRequest{
			ProductId: medA, BatchNumber: "MD-A2", ExpiryDate: "2099-12-31",
			CostPrice: 700, InitialQuantity: 3,
			SupplierId: sup.Msg.Supplier.Id, ReceivedAt: "2026-05-22",
		}, wh))
	require.NoError(t, err)
	got2, err := env.Products.GetProduct(ctx, whReq(env, t,
		&inventoryifacev1.GetProductRequest{Id: medA}, wh))
	require.NoError(t, err)
	require.Equal(t, int64(700), got2.Msg.Product.ReferenceCost, "reference cost = latest batch (700)")

	// medB has no batch → no last-restock, no reference cost.
	gotB, err := env.Products.GetProduct(ctx, whReq(env, t,
		&inventoryifacev1.GetProductRequest{Id: medB}, wh))
	require.NoError(t, err)
	require.Empty(t, gotB.Msg.Product.LastRestockDate)
	require.Empty(t, gotB.Msg.Product.LastRestockSupplier)
	require.Equal(t, int64(0), gotB.Msg.Product.ReferenceCost, "no batch → reference cost 0")

	// ListMovements{product_id: medA} → at least the PURCHASE movement; all rows
	// belong to medA's batch (medB has none). Movements are scoped to the active
	// warehouse, so query the same warehouse the batch was seeded into.
	mv, err := env.Stock.ListMovements(ctx, whReq(env, t,
		&inventoryifacev1.ListMovementsRequest{ProductId: medA}, wh))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(mv.Msg.Movements), 1)
	require.GreaterOrEqual(t, mv.Msg.Total, int32(1))

	mvB, err := env.Stock.ListMovements(ctx, whReq(env, t,
		&inventoryifacev1.ListMovementsRequest{ProductId: medB}, wh))
	require.NoError(t, err)
	require.Equal(t, int32(0), mvB.Msg.Total, "medB has no movements")
}
