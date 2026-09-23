package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (e *poEnv) seedManufacturer(t *testing.T, code, name string) string {
	t.Helper()
	m := model.Manufacturer{Code: code, Name: name}
	require.NoError(t, e.db.Create(&m).Error)
	return m.ID
}

// createPOFrom orders one line and records which pabrik it is sourced from.
func (e *poEnv) createPOFrom(t *testing.T, supplierID, productID, manufacturerID string, qty int32, unitCost int64) *purchasingifacev1.PurchaseOrder {
	t.Helper()
	resp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supplierID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{{
			ProductId:      productID,
			OrderedQty:     qty,
			UnitCostPrice:  unitCost,
			ManufacturerId: manufacturerID,
		}},
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Order.Items)
	return resp.Msg.Order
}

// The whole chain, and the reason the field exists: what the buyer recorded on
// the ORDER becomes a permanent fact about the physical LOT at receive. Without
// this step the line-level maker would be a note that dies at the warehouse
// door, which is precisely the objection the single-column model could not
// answer.
func TestCreateReceipt_StampsTheLineManufacturerOntoTheLot(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "SUP-MFR-1", "PT Distributor")
	prod := e.seedProduct(t, "SKU-MFR-PO-1", "Paracetamol 500", 5000)
	kalbe := e.seedManufacturer(t, "MFR-PO-1", "Kalbe Farma")

	po := e.createPOFrom(t, sup, prod, kalbe, 10, 1000)
	require.Equal(t, kalbe, po.Items[0].ManufacturerId, "the order must carry it back")

	e.sendPO(t, po.Id)
	rcvID := e.receiveFull(t, po.Id, po.Items[0].Id, 10, "LOT-KALBE-1")
	_, batchID := e.receiptItem(t, rcvID)

	var lot model.Batch
	require.NoError(t, e.db.Where("id = ?", batchID).First(&lot).Error)
	require.NotNil(t, lot.ManufacturerID, "the lot must record who made it")
	require.Equal(t, kalbe, *lot.ManufacturerID)
}

// A line that names no pabrik must yield a lot that names none either. An
// unverified name on a lot is worse than a blank, because only the blank is
// recognisable as not-recorded.
func TestCreateReceipt_LeavesTheLotMakerBlankWhenTheLineNamesNone(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "SUP-MFR-2", "PT Distributor")
	prod := e.seedProduct(t, "SKU-MFR-PO-2", "Amoxsan", 7500)

	po := e.createPO(t, sup, prod, 5, 2000)
	e.sendPO(t, po.Id)
	rcvID := e.receiveFull(t, po.Id, po.Items[0].Id, 5, "LOT-NOMAKER")
	_, batchID := e.receiptItem(t, rcvID)

	var lot model.Batch
	require.NoError(t, e.db.Where("id = ?", batchID).First(&lot).Error)
	require.Nil(t, lot.ManufacturerID)
}

// The restock log is the per-product history of the same event, and it already
// snapshots the line's price / qty / discount rather than joining back to an
// order that can still be edited. The maker belongs on the same footing: this
// is what lets the product page answer WHOSE tablets arrived on a given buy,
// not merely which distributor sold them.
func TestCreateReceipt_RecordsTheMakerOnTheRestockLog(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "SUP-MFR-LOG", "PT Distributor")
	prod := e.seedProduct(t, "SKU-MFR-LOG", "Paracetamol 500", 5000)
	kalbe := e.seedManufacturer(t, "MFR-LOG-1", "Kalbe Farma")
	dexa := e.seedManufacturer(t, "MFR-LOG-2", "Dexa Medica")

	// Same product, same supplier, two buys from two different factories -- the
	// case a single column on the product could not record at all.
	po1 := e.createPOFrom(t, sup, prod, kalbe, 10, 1000)
	e.sendPO(t, po1.Id)
	e.receiveFull(t, po1.Id, po1.Items[0].Id, 10, "LOT-LOG-K")

	po2 := e.createPOFrom(t, sup, prod, dexa, 4, 1100)
	e.sendPO(t, po2.Id)
	e.receiveFull(t, po2.Id, po2.Items[0].Id, 4, "LOT-LOG-D")

	var logs []model.ProductRestockLog
	require.NoError(t, e.db.Where("product_id = ?", prod).
		Order("restock_arrived_at ASC, created_at ASC").Find(&logs).Error)
	require.Len(t, logs, 2)
	require.NotNil(t, logs[0].ManufacturerID)
	require.Equal(t, kalbe, *logs[0].ManufacturerID)
	require.NotNil(t, logs[1].ManufacturerID)
	require.Equal(t, dexa, *logs[1].ManufacturerID)
}

// A buy that never recorded a maker leaves the column NULL rather than
// borrowing the product's usual one, for the same reason the lot does: a guess
// reads exactly like a fact once it is stored.
func TestCreateReceipt_RestockLogMakerBlankWhenTheLineNamesNone(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "SUP-MFR-LOG-2", "PT Distributor")
	prod := e.seedProduct(t, "SKU-MFR-LOG-2", "Amoxsan", 7500)

	po := e.createPO(t, sup, prod, 5, 2000)
	e.sendPO(t, po.Id)
	e.receiveFull(t, po.Id, po.Items[0].Id, 5, "LOT-LOG-NONE")

	var log model.ProductRestockLog
	require.NoError(t, e.db.Where("product_id = ?", prod).First(&log).Error)
	require.Nil(t, log.ManufacturerID)
}
// Two lots of the SAME product from different pabrik -- the state the old
// single-column model could not represent at all.
func TestCreateReceipt_TwoMakersForOneProduct(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "SUP-MFR-3", "PT Distributor")
	prod := e.seedProduct(t, "SKU-MFR-PO-3", "Paracetamol 500", 5000)
	kalbe := e.seedManufacturer(t, "MFR-PO-2", "Kalbe Farma")
	dexa := e.seedManufacturer(t, "MFR-PO-3", "Dexa Medica")

	makers := map[string]string{}
	for _, m := range []struct{ id, lot string }{{kalbe, "LOT-K"}, {dexa, "LOT-D"}} {
		po := e.createPOFrom(t, sup, prod, m.id, 4, 1000)
		e.sendPO(t, po.Id)
		rcvID := e.receiveFull(t, po.Id, po.Items[0].Id, 4, m.lot)
		_, batchID := e.receiptItem(t, rcvID)
		var lot model.Batch
		require.NoError(t, e.db.Where("id = ?", batchID).First(&lot).Error)
		require.NotNil(t, lot.ManufacturerID)
		makers[lot.BatchNumber] = *lot.ManufacturerID
	}
	require.Equal(t, kalbe, makers["LOT-K"])
	require.Equal(t, dexa, makers["LOT-D"], "each lot keeps its own maker")
}

// The restock list's Pabrik filter, and the filter-parity HARD RULE: the stat
// row must describe the SAME set as the table under it, which is why both
// handlers share applyPOFilters.
func TestListPurchaseOrders_FiltersByManufacturer_AndSummaryAgrees(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	sup := e.seedSupplier(t, "SUP-MFR-4", "PT Distributor")
	prod := e.seedProduct(t, "SKU-MFR-PO-4", "Paracetamol 500", 5000)
	kalbe := e.seedManufacturer(t, "MFR-PO-4", "Kalbe Farma")
	dexa := e.seedManufacturer(t, "MFR-PO-5", "Dexa Medica")

	e.createPOFrom(t, sup, prod, kalbe, 10, 1000)
	e.createPOFrom(t, sup, prod, kalbe, 3, 1000)
	e.createPOFrom(t, sup, prod, dexa, 7, 1000)
	e.createPO(t, sup, prod, 2, 1000) // no maker recorded

	all, err := e.pos.ListPurchaseOrders(e.ctx, connect.NewRequest(&purchasingifacev1.ListPurchaseOrdersRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(4), all.Msg.Total)

	list, err := e.pos.ListPurchaseOrders(e.ctx, connect.NewRequest(&purchasingifacev1.ListPurchaseOrdersRequest{
		ManufacturerId: kalbe,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(2), list.Msg.Total)

	sum, err := e.pos.GetPurchaseOrdersSummary(e.ctx, connect.NewRequest(&purchasingifacev1.GetPurchaseOrdersSummaryRequest{
		ManufacturerId: kalbe,
	}))
	require.NoError(t, err)
	require.EqualValues(t, list.Msg.Total, sum.Msg.OrderCount,
		"the stat row and the table must describe the same set")
	// 10 + 3 base units ordered from this pabrik, and nothing from the others.
	require.EqualValues(t, 13, sum.Msg.ItemCount)
}
