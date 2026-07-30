package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// Order creation is the money path for grosir: the tier price must survive
// DRAFT -> COMPLETED with the sale number, totals and stock ledger all agreeing.
func TestCompleteSale_GrosirOrder(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	prodID, _ := ladder(t, db, "GO1")
	seedStock(t, db, prodID, ownerID, 500)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 12}))
	require.NoError(t, err)

	resp, err := svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    102000,
	}))
	require.NoError(t, err)
	sale := resp.Msg.Sale
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_COMPLETED, sale.Status)
	require.NotEmpty(t, sale.SaleNo)
	require.NotZero(t, sale.CompletedAt)
	require.Equal(t, int64(102000), sale.Total, "grosir total, not 12 x 10.000")

	// The persisted line keeps the full pricing provenance.
	var item model.SaleItem
	require.NoError(t, db.Where("sale_id = ?", saleID).First(&item).Error)
	require.Equal(t, int64(8500), item.UnitPriceSnapshot)
	require.Equal(t, int64(10000), item.ListPriceSnapshot)
	require.Equal(t, int32(12), item.TierMinQty)
}

// Stock allocation is driven by base_qty and must be untouched by the tier
// price: 12 units out, as one negative SALE movement linked to the line.
func TestCompleteSale_GrosirConsumesBaseQty(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	prodID, _ := ladder(t, db, "GO2")
	seedStock(t, db, prodID, ownerID, 500)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 12}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 102000,
	}))
	require.NoError(t, err)

	var item model.SaleItem
	require.NoError(t, db.Where("sale_id = ?", saleID).First(&item).Error)
	require.Equal(t, int32(12), item.BaseQty)

	var moves []model.StockMovement
	require.NoError(t, db.Where("sale_item_id = ?", item.ID).Find(&moves).Error)
	require.NotEmpty(t, moves, "SALE movements must link back to the line")
	var consumed int32
	for _, m := range moves {
		require.Equal(t, "SALE", m.Type)
		require.Negative(t, m.Qty)
		consumed += -m.Qty
	}
	require.Equal(t, int32(12), consumed, "qty consumed follows base_qty, not the price")
}

// Grosir composes with every other money input: a normal line, a manual line
// discount, a cart discount and a service fee all still add up.
func TestCompleteSale_GrosirMixedOrderTotals(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	grosirID, _ := ladder(t, db, "GO3")
	plainID := seedProduct(t, db, "GO3-plain", "Sabun", 5000)
	seedStock(t, db, grosirID, ownerID, 500)
	seedStock(t, db, plainID, ownerID, 500)
	saleID := startDraft(t, svc, ctx)

	// Grosir line: 12 x 8.500 = 102.000
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: grosirID, Qty: 12}))
	require.NoError(t, err)
	// Plain line: 3 x 5.000 = 15.000, then a manual FIXED 1.000 off -> 14.000
	addPlain, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: plainID, Qty: 3}))
	require.NoError(t, err)
	var plainItemID string
	for _, it := range addPlain.Msg.Sale.Items {
		if it.ProductId == plainID {
			plainItemID = it.Id
		}
	}
	require.NotEmpty(t, plainItemID)
	_, err = svc.SetLineDiscount(ctx, connect.NewRequest(&posifacev1.SetLineDiscountRequest{
		SaleId: saleID, ItemId: plainItemID, DiscountType: "FIXED", DiscountValue: 1000,
	}))
	require.NoError(t, err)
	// Cart discount 10% of subtotal 116.000 = 11.600 -> 104.400
	_, err = svc.SetCartDiscount(ctx, connect.NewRequest(&posifacev1.SetCartDiscountRequest{
		SaleId: saleID, DiscountType: "PERCENT", DiscountValue: 1000,
	}))
	require.NoError(t, err)
	// Service fee +2.000 -> 106.400
	fee, err := svc.SetServiceFee(ctx, connect.NewRequest(&posifacev1.SetServiceFeeRequest{SaleId: saleID, BiayaJasa: 2000}))
	require.NoError(t, err)
	require.Equal(t, int64(116000), fee.Msg.Sale.Subtotal)
	require.Equal(t, int64(11600), fee.Msg.Sale.CartDiscount)
	require.Equal(t, int64(106400), fee.Msg.Sale.Total)

	resp, err := svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 106400,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(106400), resp.Msg.Sale.Total)
}

// Crossing the tier doesn't loosen the stock guard: the whole tx rolls back and
// the draft is left untouched, price fields included.
func TestCompleteSale_GrosirInsufficientStock(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	prodID, _ := ladder(t, db, "GO4")
	seedStock(t, db, prodID, ownerID, 5) // far short of 12
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 12}))
	require.NoError(t, err)

	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 102000,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	var sale model.Sale
	require.NoError(t, db.Where("id = ?", saleID).First(&sale).Error)
	require.Equal(t, "DRAFT", sale.Status)
	require.Nil(t, sale.SaleNo)
	var item model.SaleItem
	require.NoError(t, db.Where("sale_id = ?", saleID).First(&item).Error)
	require.Equal(t, int64(8500), item.UnitPriceSnapshot, "draft pricing untouched by the failed completion")
	var n int64
	require.NoError(t, db.Model(&model.StockMovement{}).Where("sale_item_id = ?", item.ID).Count(&n).Error)
	require.Zero(t, n, "no movements written")
}

// The completed order reads back with its grosir fields, and the summary
// aggregate agrees with the list (both go through applySaleFilters).
func TestCompleteSale_GrosirOrderReadBack(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	prodID, _ := ladder(t, db, "GO5")
	seedStock(t, db, prodID, ownerID, 500)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 12}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 102000,
	}))
	require.NoError(t, err)

	got, err := svc.GetSale(ctx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: saleID}))
	require.NoError(t, err)
	it := got.Msg.Sale.Items[0]
	require.Equal(t, int64(8500), it.UnitPriceSnapshot)
	require.Equal(t, int64(10000), it.ListPriceSnapshot)
	require.Equal(t, int32(12), it.TierMinQty)

	list, err := svc.ListSales(ctx, connect.NewRequest(&posifacev1.ListSalesRequest{
		Status: posifacev1.SaleStatus_SALE_STATUS_COMPLETED,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), list.Msg.Total)
	require.Equal(t, int64(102000), list.Msg.Sales[0].Total)

	sum, err := svc.GetSalesSummary(ctx, connect.NewRequest(&posifacev1.GetSalesSummaryRequest{
		Status: posifacev1.SaleStatus_SALE_STATUS_COMPLETED,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(102000), sum.Msg.Revenue, "summary must agree with the list")
}

// A refund returns what was actually charged — the grosir total, not the list.
func TestRefundSale_GrosirOrder(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	prodID, _ := ladder(t, db, "GO6")
	seedStock(t, db, prodID, ownerID, 500)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: prodID, Qty: 12}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 102000,
	}))
	require.NoError(t, err)

	resp, err := svc.RefundSale(ctx, connect.NewRequest(&posifacev1.RefundSaleRequest{
		SaleId: saleID, Reason: "grosir return", Restock: true,
	}))
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_REFUNDED, resp.Msg.Sale.Status)
	require.Equal(t, int64(102000), resp.Msg.Sale.RefundAmount, "refund the grosir total, not 120.000")
}
