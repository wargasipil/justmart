package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

// The faktur prices a box, so the line is priced per box, exactly. Before
// pack_cost_price the form sent a ROUNDED per-base cost (65.000 / 30 = 2.167)
// and the server rebuilt the line from it: 50 box x 30 x 2.167 = 3.250.500,
// Rp 500 over an invoice that says 3.250.000 -- and the supplier's debt with it.

// packLine is the shop's real example: a box of 30 at Rp 65.000, 50 boxes. The
// per-base cost rides along the way the frontend sends it (rounded).
func packLine(productID, boxUnitID string) *purchasingifacev1.PurchaseOrderItemInput {
	return &purchasingifacev1.PurchaseOrderItemInput{
		ProductId:     productID,
		ProductUnitId: boxUnitID,
		OrderedQty:    50,
		PackCostPrice: 65_000,
		UnitCostPrice: 2_167,
	}
}

func TestCreatePurchaseOrder_PackCostIsExact(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-PACK-1", "Pack supplier")
	prodID, boxID := e.seedProductWithBox(t, "pack-sku-1", "Pack product", 3_000, 30)

	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items:      []*purchasingifacev1.PurchaseOrderItemInput{packLine(prodID, boxID)},
	}))
	require.NoError(t, err)
	o := resp.Msg.Order
	require.Len(t, o.Items, 1)
	it := o.Items[0]
	require.Equal(t, int32(1_500), it.OrderedQty, "stored in base units")
	require.Equal(t, int64(3_250_000), it.Subtotal, "50 x 65.000, not 1.500 x 2.167")
	require.Equal(t, int64(3_250_000), o.Subtotal)
	require.Equal(t, int64(3_250_000), o.OrderedTotal)
	require.Equal(t, int64(3_250_000), o.Outstanding, "the supplier's debt is the faktur")
	require.Equal(t, int64(65_000), it.PackCostPrice, "the typed pack price is stored as typed")
	require.Equal(t, int64(2_167), it.UnitCostPrice, "per-base is derived, half-up, for legacy readers")

	// GetPurchaseOrder reads the same columns back.
	got := e.getPO(t, o.Id)
	require.Equal(t, int64(65_000), got.Items[0].PackCostPrice)
	require.Equal(t, int64(2_167), got.Items[0].UnitCostPrice)
	require.Equal(t, int64(3_250_000), got.OrderedTotal)
}

func TestCreatePurchaseOrder_PackCostWithPPN(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-PACK-PPN", "Pack PPN supplier")
	prodID, boxID := e.seedProductWithBox(t, "pack-sku-ppn", "Pack PPN product", 3_000, 30)

	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		PpnEnabled: true,
		PpnRate:    11,
		Items:      []*purchasingifacev1.PurchaseOrderItemInput{packLine(prodID, boxID)},
	}))
	require.NoError(t, err)
	o := resp.Msg.Order
	require.Equal(t, int64(3_250_000), o.Subtotal)
	require.Equal(t, int64(357_500), o.PpnAmount)
	require.Equal(t, int64(3_607_500), o.OrderedTotal)
}

// The server derives unit_cost_price itself on the exact path: a client that
// sends a stale or missing per-base figure cannot make the two columns disagree.
func TestCreatePurchaseOrder_PackCostDerivesUnitCost(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-PACK-DRV", "Pack derive supplier")
	prodID, boxID := e.seedProductWithBox(t, "pack-sku-drv", "Pack derive product", 3_000, 30)

	in := packLine(prodID, boxID)
	in.UnitCostPrice = 0
	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items:      []*purchasingifacev1.PurchaseOrderItemInput{in},
	}))
	require.NoError(t, err)
	require.Equal(t, int64(2_167), resp.Msg.Order.Items[0].UnitCostPrice)
	require.Equal(t, int64(3_250_000), resp.Msg.Order.Items[0].Subtotal)
}

// A client that sends no pack price (anything built before the field) gets
// exactly the old per-base result, rounding error included -- an order saved
// that way must not silently re-price.
func TestCreatePurchaseOrder_LegacyLineKeepsPerBaseMath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-PACK-LEG", "Legacy supplier")
	prodID, boxID := e.seedProductWithBox(t, "pack-sku-leg", "Legacy product", 3_000, 30)

	in := packLine(prodID, boxID)
	in.PackCostPrice = 0
	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items:      []*purchasingifacev1.PurchaseOrderItemInput{in},
	}))
	require.NoError(t, err)
	it := resp.Msg.Order.Items[0]
	require.Equal(t, int64(3_250_500), it.Subtotal, "1.500 x 2.167, as before")
	require.Equal(t, int64(2_167), it.UnitCostPrice)
	require.Zero(t, it.PackCostPrice)
	require.Zero(t, e.getPO(t, resp.Msg.Order.Id).Items[0].PackCostPrice)
}

// Discounts stay exact on the pack path: per item, each box's 65.000 is
// discounted then x 50; per line, the exact 3.250.000 is.
func TestCreatePurchaseOrder_PackCostPercentDiscounts(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-PACK-DSC", "Pack discount supplier")
	prodID, boxID := e.seedProductWithBox(t, "pack-sku-dsc", "Pack discount product", 3_000, 30)

	perItem := packLine(prodID, boxID)
	perItem.DiscountType, perItem.DiscountValue, perItem.DiscountPerItem = "PERCENT", 250, true // 2.5%
	perLine := packLine(prodID, boxID)
	perLine.DiscountType, perLine.DiscountValue = "PERCENT", 250

	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items:      []*purchasingifacev1.PurchaseOrderItemInput{perItem, perLine},
	}))
	require.NoError(t, err)
	items := resp.Msg.Order.Items
	require.Len(t, items, 2)
	byPerItem := map[bool]int64{}
	for _, it := range items {
		byPerItem[it.DiscountPerItem] = it.Subtotal
	}
	// 65.000 x 2.5% = 1.625 off each box -> 63.375 x 50.
	require.Equal(t, int64(3_168_750), byPerItem[true])
	// 3.250.000 x 2.5% = 81.250 off the line.
	require.Equal(t, int64(3_168_750), byPerItem[false])
}

func TestCreatePurchaseOrder_NegativePackCostRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-PACK-NEG", "Pack negative supplier")
	prodID, boxID := e.seedProductWithBox(t, "pack-sku-neg", "Pack negative product", 3_000, 30)

	in := packLine(prodID, boxID)
	in.PackCostPrice = -1
	_, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items:      []*purchasingifacev1.PurchaseOrderItemInput{in},
	}))
	requireReceiptToken(t, err, connect.CodeInvalidArgument, "purchasing.cost_negative")
}
