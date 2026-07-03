package purchasing_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func TestCreatePurchaseOrder_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CPO", "PO supplier")
	prodID := e.seedProduct(t, "cpo-sku", "CPO product", 1000)

	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Note:       "first order",
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 5, UnitCostPrice: 800},
		},
	}))
	require.NoError(t, err)
	o := resp.Msg.Order
	require.NotNil(t, o)
	require.NotEmpty(t, o.Id)
	require.NotEmpty(t, o.PoNo) // assigned PO-YYYY-NNNN
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_DRAFT, o.Status)
	require.Equal(t, supID, o.SupplierId)
	require.Equal(t, "first order", o.Note)
	require.Equal(t, e.ownerID, o.CreatedBy)
	require.NotEmpty(t, o.WarehouseId) // resolved to MAIN
	require.Len(t, o.Items, 1)
	require.Equal(t, prodID, o.Items[0].ProductId)
	require.Equal(t, int32(5), o.Items[0].OrderedQty)
	require.Equal(t, int64(800), o.Items[0].UnitCostPrice)
	require.Equal(t, int64(4000), o.Subtotal) // 5 × 800
	require.Equal(t, int64(4000), o.OrderedTotal)
}

func TestCreatePurchaseOrder_FixedLineDiscount(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CPO-FIX", "Fixed disc supplier")
	prodID := e.seedProduct(t, "cpo-fix-sku", "Fixed disc product", 1000)

	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 5, UnitCostPrice: 800, DiscountType: "FIXED", DiscountValue: 500},
		},
	}))
	require.NoError(t, err)
	o := resp.Msg.Order
	require.Len(t, o.Items, 1)
	require.Equal(t, "FIXED", o.Items[0].DiscountType)
	require.Equal(t, int64(500), o.Items[0].DiscountValue)
	require.Equal(t, int64(3500), o.Items[0].Subtotal) // 5*800=4000 − 500
	require.Equal(t, int64(3500), o.Subtotal)
	require.Equal(t, int64(3500), o.OrderedTotal)
}

func TestCreatePurchaseOrder_PercentLineDiscountDecimal(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CPO-PCT", "Pct disc supplier")
	prodID := e.seedProduct(t, "cpo-pct-sku", "Pct disc product", 1000)

	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			// 12.5% = 1250 basis points; gross 10*1000=10000, disc 1250 → net 8750.
			{ProductId: prodID, OrderedQty: 10, UnitCostPrice: 1000, DiscountType: "PERCENT", DiscountValue: 1250},
		},
	}))
	require.NoError(t, err)
	o := resp.Msg.Order
	require.Len(t, o.Items, 1)
	require.Equal(t, "PERCENT", o.Items[0].DiscountType)
	require.Equal(t, int64(1250), o.Items[0].DiscountValue)
	require.Equal(t, int64(8750), o.Items[0].Subtotal)
	require.Equal(t, int64(8750), o.Subtotal)
	require.Equal(t, int64(8750), o.OrderedTotal)
}

func TestCreatePurchaseOrder_FixedPerItemDiscount(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CPO-FPI", "Fixed/item supplier")
	prodID := e.seedProduct(t, "cpo-fpi-sku", "Fixed/item product", 1000)

	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			// 5 items @ 800; Rp 200 off EACH item → disc 1000, net 3000.
			{ProductId: prodID, OrderedQty: 5, UnitCostPrice: 800, DiscountType: "FIXED", DiscountValue: 200, DiscountPerItem: true},
		},
	}))
	require.NoError(t, err)
	o := resp.Msg.Order
	require.Len(t, o.Items, 1)
	require.Equal(t, "FIXED", o.Items[0].DiscountType)
	require.True(t, o.Items[0].DiscountPerItem)
	require.Equal(t, int64(3000), o.Items[0].Subtotal) // 5*800 − 5*200
	require.Equal(t, int64(3000), o.OrderedTotal)
}

func TestCreatePurchaseOrder_FixedPerItemClampedToItemGross(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CPO-FPC", "Fixed/item clamp supplier")
	prodID := e.seedProduct(t, "cpo-fpc-sku", "Clamp product", 1000)

	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			// Rp 150 off each, but each item only costs 100 → per-item disc clamps
			// to 100, total disc 400, net 0 (never negative).
			{ProductId: prodID, OrderedQty: 4, UnitCostPrice: 100, DiscountType: "FIXED", DiscountValue: 150, DiscountPerItem: true},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, int64(0), resp.Msg.Order.Items[0].Subtotal)
}

func TestCreatePurchaseOrder_PercentPerItemVsLineRounding(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CPO-PPI", "Pct/item supplier")
	prodID := e.seedProduct(t, "cpo-ppi-sku", "Pct/item product", 1000)

	// 3 items @ 105, 5% (500 bp). Per-item: round(105*5% )=round(5.25)=5 → 15 off → net 300.
	// Line-level: round(315*5%)=round(15.75)=16 off → net 299. The two differ by 1.
	perItem, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 3, UnitCostPrice: 105, DiscountType: "PERCENT", DiscountValue: 500, DiscountPerItem: true},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, int64(300), perItem.Msg.Order.Items[0].Subtotal)
	require.True(t, perItem.Msg.Order.Items[0].DiscountPerItem)

	line, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 3, UnitCostPrice: 105, DiscountType: "PERCENT", DiscountValue: 500, DiscountPerItem: false},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, int64(299), line.Msg.Order.Items[0].Subtotal)
	require.False(t, line.Msg.Order.Items[0].DiscountPerItem)
}

func TestCreatePurchaseOrder_PerItemUsesChosenUnitNotBase(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CPO-BOX", "Box supplier")
	prodID, boxUnitID := e.seedProductWithBox(t, "cpo-box-sku", "Box product", 1000, 100)

	// 2 boxes (factor 100) @ 10/base = gross 2000. perItemGross = gross/2 boxes =
	// 1000/box. Rp 50 off EACH BOX → disc 100, net 1900. Proves chosenQty (2 boxes)
	// drives per-item division, not baseQty (200).
	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, ProductUnitId: boxUnitID, OrderedQty: 2, UnitCostPrice: 10, DiscountType: "FIXED", DiscountValue: 50, DiscountPerItem: true},
		},
	}))
	require.NoError(t, err)
	o := resp.Msg.Order
	require.Equal(t, int32(200), o.Items[0].OrderedQty) // stored in base units
	require.Equal(t, int64(1900), o.Items[0].Subtotal)
}

func TestCreatePurchaseOrder_NoItems(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CPO2", "PO supplier 2")

	_, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		// no items -> InvalidArgument
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreatePurchaseOrder_Unauthenticated(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	// No principal in ctx -> auth.MustPrincipal returns CodeUnauthenticated.
	_, err := e.pos.CreatePurchaseOrder(context.Background(), connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: "anything",
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: "x", OrderedQty: 1, UnitCostPrice: 1},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
