package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
	batchsvc "github.com/justmart/backend/internal/service/batch"
)

// createPackPO creates + sends the 50 x box-of-30 @ 65.000 order (see packLine)
// and returns it.
func (e *poEnv) createPackPO(t *testing.T, code string, ppn bool, line func(prodID, boxID string) *purchasingifacev1.PurchaseOrderItemInput) *purchasingifacev1.PurchaseOrder {
	t.Helper()
	supID := e.seedSupplier(t, code, code+" supplier")
	prodID, boxID := e.seedProductWithBox(t, code+"-sku", code+" product", 3_000, 30)
	req := &purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items:      []*purchasingifacev1.PurchaseOrderItemInput{line(prodID, boxID)},
	}
	if ppn {
		req.PpnEnabled, req.PpnRate = true, 11
	}
	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(req))
	require.NoError(t, err)
	e.sendPO(t, resp.Msg.Order.Id)
	return resp.Msg.Order
}

func (e *poEnv) batchByNumber(t *testing.T, batchNo string) model.Batch {
	t.Helper()
	var b model.Batch
	require.NoError(t, e.db.Where("batch_number = ?", batchNo).First(&b).Error)
	return b
}

// The lot remembers what ONE box cost, exactly -- on the same NET basis as its
// per-base cost_price -- so the product page can say "Rp 65.000 / box" where
// cost_price x 30 would say 65.010.
func TestCreateReceipt_StampsExactPackCost(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	po := e.createPackPO(t, "RCV-PACK", false, packLine)

	// Qty is in the PO line's unit (box): 50 boxes = the whole line.
	e.receiveFull(t, po.Id, po.Items[0].Id, 50, "RCV-PACK-B1")

	b := e.batchByNumber(t, "RCV-PACK-B1")
	require.Equal(t, int64(65_000), b.CostPackPrice)
	require.Equal(t, int64(30), b.CostPackFactor)
	require.Equal(t, int64(2_167), b.CostPrice, "per-base stays NetUnitCost over the base qty")

	// The batch reads carry it.
	svc := batchsvc.NewBatchService(e.db)
	got, err := svc.GetBatch(e.ctx, connect.NewRequest(&inventoryifacev1.GetBatchRequest{Id: b.ID}))
	require.NoError(t, err)
	require.Equal(t, int64(65_000), got.Msg.Batch.CostPackPrice)
	require.Equal(t, int64(30), got.Msg.Batch.CostPackFactor)
}

// PPN is capitalized into the pack cost exactly as into cost_price, and rounded
// once per pack: 3.250.000 x 1.11 / 50 = 72.150.
func TestCreateReceipt_PackCostCapitalizesPPN(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	po := e.createPackPO(t, "RCV-PACK-PPN", true, packLine)

	e.receiveFull(t, po.Id, po.Items[0].Id, 50, "RCV-PACK-PPN-B1")

	b := e.batchByNumber(t, "RCV-PACK-PPN-B1")
	require.Equal(t, int64(72_150), b.CostPackPrice)
	require.Equal(t, int64(30), b.CostPackFactor)
	require.Equal(t, int64(2_405), b.CostPrice)
}

// A partial delivery stamps the same per-pack figure: it is a property of the
// order line, not of how many boxes arrived this time.
func TestCreateReceipt_PartialDeliveryStampsSamePackCost(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	po := e.createPackPO(t, "RCV-PACK-PART", false, packLine)

	e.receiveFull(t, po.Id, po.Items[0].Id, 20, "RCV-PACK-PART-B1")

	b := e.batchByNumber(t, "RCV-PACK-PART-B1")
	require.Equal(t, int64(65_000), b.CostPackPrice)
	require.Equal(t, int64(30), b.CostPackFactor)
}

// A line saved without a pack price has no exact figure to stamp: the lot keeps
// 0/0 and readers derive from cost_price, as for every older lot.
func TestCreateReceipt_LegacyLineStampsNoPackCost(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	po := e.createPackPO(t, "RCV-PACK-LEG", false, func(prodID, boxID string) *purchasingifacev1.PurchaseOrderItemInput {
		in := packLine(prodID, boxID)
		in.PackCostPrice = 0
		return in
	})

	e.receiveFull(t, po.Id, po.Items[0].Id, 50, "RCV-PACK-LEG-B1")

	b := e.batchByNumber(t, "RCV-PACK-LEG-B1")
	require.Zero(t, b.CostPackPrice)
	require.Zero(t, b.CostPackFactor)
	require.Equal(t, int64(2_167), b.CostPrice)
}

// An explicit receive-time cost is a per-base override with no pack behind it,
// so the order line's pack price no longer describes the lot: 0/0.
func TestCreateReceipt_CostOverrideStampsNoPackCost(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	po := e.createPackPO(t, "RCV-PACK-OVR", false, packLine)

	_, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: po.Id,
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: po.Items[0].Id, Qty: 50, ExpiryDate: "2099-12-31",
				BatchNumber: "RCV-PACK-OVR-B1", UnitCostPrice: 2_000},
		},
	}))
	require.NoError(t, err)

	b := e.batchByNumber(t, "RCV-PACK-OVR-B1")
	require.Zero(t, b.CostPackPrice)
	require.Zero(t, b.CostPackFactor)
	require.Equal(t, int64(2_000), b.CostPrice)
}
