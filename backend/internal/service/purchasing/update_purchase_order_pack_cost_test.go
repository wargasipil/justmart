package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

// Editing a DRAFT prices its lines through the same lineGross as create: an
// order re-saved from the edit form lands on the faktur's figure, and one saved
// by a legacy client keeps its old one.
func TestUpdatePurchaseOrder_PackCostIsExact(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-UPACK", "Update pack supplier")
	prodID, boxID := e.seedProductWithBox(t, "upack-sku", "Update pack product", 3_000, 30)
	po := e.createPO(t, supID, prodID, 5, 800)

	resp, err := e.pos.UpdatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
		Id:    po.Id,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{packLine(prodID, boxID)},
	}))
	require.NoError(t, err)
	o := resp.Msg.Order
	require.Len(t, o.Items, 1)
	require.Equal(t, int64(3_250_000), o.Items[0].Subtotal)
	require.Equal(t, int64(3_250_000), o.OrderedTotal)
	require.Equal(t, int64(65_000), o.Items[0].PackCostPrice)
	require.Equal(t, int64(2_167), o.Items[0].UnitCostPrice)

	// Same lines with PPN 11%.
	resp, err = e.pos.UpdatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
		Id:         po.Id,
		PpnEnabled: true,
		PpnRate:    11,
		Items:      []*purchasingifacev1.PurchaseOrderItemInput{packLine(prodID, boxID)},
	}))
	require.NoError(t, err)
	require.Equal(t, int64(3_607_500), resp.Msg.Order.OrderedTotal)

	// A header-only edit (no items) re-totals the stored exact lines.
	resp, err = e.pos.UpdatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
		Id:   po.Id,
		Note: "header only",
	}))
	require.NoError(t, err)
	require.Equal(t, int64(3_250_000), resp.Msg.Order.OrderedTotal)
	require.Equal(t, int64(65_000), e.getPO(t, po.Id).Items[0].PackCostPrice)
}

func TestUpdatePurchaseOrder_LegacyLineKeepsPerBaseMath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-UPACK-LEG", "Update legacy supplier")
	prodID, boxID := e.seedProductWithBox(t, "upack-leg-sku", "Update legacy product", 3_000, 30)
	po := e.createPO(t, supID, prodID, 5, 800)

	in := packLine(prodID, boxID)
	in.PackCostPrice = 0
	resp, err := e.pos.UpdatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
		Id:    po.Id,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{in},
	}))
	require.NoError(t, err)
	require.Equal(t, int64(3_250_500), resp.Msg.Order.Items[0].Subtotal)
	require.Zero(t, resp.Msg.Order.Items[0].PackCostPrice)
}

func TestUpdatePurchaseOrder_NegativePackCostRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-UPACK-NEG", "Update negative supplier")
	prodID, boxID := e.seedProductWithBox(t, "upack-neg-sku", "Update negative product", 3_000, 30)
	po := e.createPO(t, supID, prodID, 5, 800)

	in := packLine(prodID, boxID)
	in.PackCostPrice = -65_000
	_, err := e.pos.UpdatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.UpdatePurchaseOrderRequest{
		Id:    po.Id,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{in},
	}))
	requireReceiptToken(t, err, connect.CodeInvalidArgument, "purchasing.cost_negative")
}
