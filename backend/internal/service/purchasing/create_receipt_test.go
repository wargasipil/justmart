package purchasing_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func TestCreateReceipt_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR", "Receipt supplier")
	prodID := e.seedProduct(t, "cr-sku", "CR product", 1000)
	po := e.createPO(t, supID, prodID, 10, 800)
	e.sendPO(t, po.Id) // must be SENT to receive

	resp, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: po.Id,
		InvoiceNo:       "FAK-CR-1",
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: po.Items[0].Id, Qty: 4, ExpiryDate: "2099-12-31", BatchNumber: "CR-B1"},
		},
	}))
	require.NoError(t, err)
	r := resp.Msg.Receipt
	require.NotNil(t, r)
	require.NotEmpty(t, r.Id)
	require.NotEmpty(t, r.ReceiptNo) // RCV-YYYY-NNNN
	require.Equal(t, po.Id, r.PurchaseOrderId)
	require.Equal(t, "FAK-CR-1", r.InvoiceNo)
	require.Len(t, r.Items, 1)
	require.Equal(t, int32(4), r.Items[0].Qty)

	// Partial receive moves the PO to PARTIALLY_RECEIVED and bumps received_qty.
	got, err := e.pos.GetPurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.GetPurchaseOrderRequest{Id: po.Id}))
	require.NoError(t, err)
	require.Equal(t, purchasingifacev1.POStatus_PO_STATUS_PARTIALLY_RECEIVED, got.Msg.Order.Status)
	require.Equal(t, int32(4), got.Msg.Order.Items[0].ReceivedQty)
}

// TestCreateReceipt_InheritsPOInvoiceWhenBlank pins the new behavior: the Receive
// dialog no longer asks for a faktur, so a blank request invoice_no inherits the
// PO's faktur (captured at PO create). An explicit value still wins — see
// TestCreateReceipt_HappyPath, which passes "FAK-CR-1" and asserts it survives.
func TestCreateReceipt_InheritsPOInvoiceWhenBlank(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR-INV", "Inherit-faktur supplier")
	prodID := e.seedProduct(t, "cr-inv-sku", "CR inherit product", 1000)

	// Create the PO WITH a faktur (as the create form does), then send it.
	poResp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		InvoiceNo:  "FAK-PO-9",
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 6, UnitCostPrice: 800},
		},
	}))
	require.NoError(t, err)
	po := poResp.Msg.Order
	e.sendPO(t, po.Id)

	// Receive with a BLANK invoice_no -> receipt inherits the PO's faktur.
	resp, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: po.Id,
		// InvoiceNo intentionally omitted (blank).
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: po.Items[0].Id, Qty: 6, ExpiryDate: "2099-12-31", BatchNumber: "CR-INV-B1"},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "FAK-PO-9", resp.Msg.Receipt.InvoiceNo, "blank receipt faktur must inherit the PO's")
}

// TestCreateReceipt_DiscountedCostFlowsToBatch proves a per-line PO discount
// lowers inventory cost: with no receipt cost override, the created batch's
// cost_price is the NET per-base-unit cost (gross − line discount) / qty.
func TestCreateReceipt_DiscountedCostFlowsToBatch(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR-DISC", "Disc COGS supplier")
	prodID := e.seedProduct(t, "cr-disc-sku", "Disc COGS product", 1000)

	// 10 × 1000 gross = 10000, 12.5% (1250 bps) → net 8750 → net unit = 875.
	poResp, err := e.pos.CreatePurchaseOrder(e.ctx, connect.NewRequest(&purchasingifacev1.CreatePurchaseOrderRequest{
		SupplierId: supID,
		Items: []*purchasingifacev1.PurchaseOrderItemInput{
			{ProductId: prodID, OrderedQty: 10, UnitCostPrice: 1000, DiscountType: "PERCENT", DiscountValue: 1250},
		},
	}))
	require.NoError(t, err)
	po := poResp.Msg.Order
	e.sendPO(t, po.Id)

	// Receive full, NO cost override → batch must use the NET per-unit cost.
	_ = e.receiveFull(t, po.Id, po.Items[0].Id, 10, "CR-DISC-B1")

	var batch model.Batch
	require.NoError(t, e.db.Where("batch_number = ?", "CR-DISC-B1").First(&batch).Error)
	require.Equal(t, int64(875), batch.CostPrice) // 8750 / 10 — discount reflected in COGS
}

func TestCreateReceipt_OverReceiveRejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR2", "Receipt supplier 2")
	prodID := e.seedProduct(t, "cr2-sku", "CR2 product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800)
	e.sendPO(t, po.Id)

	// Receiving more than ordered is rejected.
	_, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: po.Id,
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: po.Items[0].Id, Qty: 6, ExpiryDate: "2099-12-31", BatchNumber: "CR2-B1"},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestCreateReceipt_DraftPORejected(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-CR3", "Receipt supplier 3")
	prodID := e.seedProduct(t, "cr3-sku", "CR3 product", 1000)
	po := e.createPO(t, supID, prodID, 5, 800) // still DRAFT — not sent

	_, err := e.receipts.CreateReceipt(e.ctx, connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: po.Id,
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: po.Items[0].Id, Qty: 1, ExpiryDate: "2099-12-31", BatchNumber: "CR3-B1"},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestCreateReceipt_Unauthenticated(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	_, err := e.receipts.CreateReceipt(context.Background(), connect.NewRequest(&purchasingifacev1.CreateReceiptRequest{
		PurchaseOrderId: "x",
		Lines: []*purchasingifacev1.ReceiveLineInput{
			{PurchaseOrderItemId: "y", Qty: 1, ExpiryDate: "2099-12-31"},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
