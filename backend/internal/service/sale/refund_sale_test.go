package sale_test

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// batchQty sums all stock_movements for a batch in the MAIN warehouse (the qty
// FEFO consumes / a refund restores).
func batchQty(t *testing.T, db *gorm.DB, batchID string) int64 {
	t.Helper()
	var qty int64
	require.NoError(t, db.Model(&model.StockMovement{}).
		Where("batch_id = ? AND warehouse_id = ?", batchID, mainWarehouseID).
		Select("COALESCE(SUM(qty), 0)").Scan(&qty).Error)
	return qty
}

func TestRefundSale_RestockReturnsStock(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "rf-1", "Paracetamol", 2000)
	batchID := seedStock(t, db, productID, ownerID, 50)
	require.Equal(t, int64(50), batchQty(t, db, batchID))

	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: productID, Qty: 3}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 6000,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(47), batchQty(t, db, batchID)) // 50 - 3 sold

	resp, err := svc.RefundSale(ctx, connect.NewRequest(&posifacev1.RefundSaleRequest{
		SaleId: saleID, Reason: "customer changed mind", Restock: true,
	}))
	require.NoError(t, err)
	sale := resp.Msg.Sale
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_REFUNDED, sale.Status)
	require.NotZero(t, sale.RefundedAt)
	require.Equal(t, int64(6000), sale.RefundAmount) // = total
	require.Equal(t, "customer changed mind", sale.RefundReason)
	require.True(t, sale.RefundRestocked)

	// Goods are back in stock, and a RETURN movement records it.
	require.Equal(t, int64(50), batchQty(t, db, batchID))
	var returnCount int64
	require.NoError(t, db.Model(&model.StockMovement{}).
		Where("batch_id = ? AND type = ?", batchID, "RETURN").Count(&returnCount).Error)
	require.Equal(t, int64(1), returnCount)
}

func TestRefundSale_MoneyOnlyLeavesStock(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "rf-2", "Vitamin", 1000)
	batchID := seedStock(t, db, productID, ownerID, 50)

	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: productID, Qty: 5}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 5000,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(45), batchQty(t, db, batchID))

	resp, err := svc.RefundSale(ctx, connect.NewRequest(&posifacev1.RefundSaleRequest{
		SaleId: saleID, Restock: false,
	}))
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_REFUNDED, resp.Msg.Sale.Status)
	require.False(t, resp.Msg.Sale.RefundRestocked)

	// Stock unchanged (no RETURN movement) — money-only refund.
	require.Equal(t, int64(45), batchQty(t, db, batchID))
	var returnCount int64
	require.NoError(t, db.Model(&model.StockMovement{}).
		Where("batch_id = ? AND type = ?", batchID, "RETURN").Count(&returnCount).Error)
	require.Equal(t, int64(0), returnCount)
}

func TestRefundSale_RejectsNonCompleted(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "rf-3", "Item", 1000)
	seedStock(t, db, productID, ownerID, 50)

	// DRAFT can't be refunded.
	draftID := startDraft(t, svc, ctx)
	_, err := svc.RefundSale(ctx, connect.NewRequest(&posifacev1.RefundSaleRequest{SaleId: draftID}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	// Complete then refund once (ok), refund again → rejected (terminal).
	_, err = svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: draftID, ProductId: productID, Qty: 1}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: draftID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 1000,
	}))
	require.NoError(t, err)
	_, err = svc.RefundSale(ctx, connect.NewRequest(&posifacev1.RefundSaleRequest{SaleId: draftID, Restock: true}))
	require.NoError(t, err)
	_, err = svc.RefundSale(ctx, connect.NewRequest(&posifacev1.RefundSaleRequest{SaleId: draftID, Restock: true}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// A sale completed more than a day ago can no longer be refunded.
func TestRefundSale_RejectsAfterOneDay(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "rf-old", "Item", 1000)
	seedStock(t, db, productID, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: productID, Qty: 1}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 1000,
	}))
	require.NoError(t, err)

	// Backdate completion to 2 days ago → outside the 1-day refund window.
	require.NoError(t, db.Model(&model.Sale{}).Where("id = ?", saleID).
		Update("completed_at", time.Now().Add(-48*time.Hour)).Error)

	_, err = svc.RefundSale(ctx, connect.NewRequest(&posifacev1.RefundSaleRequest{SaleId: saleID, Restock: true}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// A refunded sale drops out of the sales summary (status filter excludes it).
func TestRefundSale_ExcludedFromSummary(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "rf-4", "Item", 2000)
	seedStock(t, db, productID, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: productID, Qty: 2}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 4000,
	}))
	require.NoError(t, err)

	before, err := svc.GetSalesSummary(ctx, connect.NewRequest(&posifacev1.GetSalesSummaryRequest{
		Status: posifacev1.SaleStatus_SALE_STATUS_COMPLETED,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(4000), before.Msg.Revenue)
	require.Equal(t, int64(1), before.Msg.SaleCount)

	_, err = svc.RefundSale(ctx, connect.NewRequest(&posifacev1.RefundSaleRequest{SaleId: saleID, Restock: true}))
	require.NoError(t, err)

	after, err := svc.GetSalesSummary(ctx, connect.NewRequest(&posifacev1.GetSalesSummaryRequest{
		Status: posifacev1.SaleStatus_SALE_STATUS_COMPLETED,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(0), after.Msg.Revenue) // refunded sale no longer counts
	require.Equal(t, int64(0), after.Msg.SaleCount)
}

// Pharmacy: a restock refund reverses prescription dispensing back to 0.
func TestRefundSale_PharmacyReversesRxDispensing(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	productID := seedRxProduct(t, db, "rf-rx", "Amoxicillin", 3000)
	seedStock(t, db, productID, ownerID, 50)
	customerID := seedCustomer(t, db, "Pasien")
	rxID := seedPrescription(t, db, customerID, ownerID, productID, 10)

	saleID := startDraft(t, svc, ctx)
	_, err := svc.SetSaleCustomer(ctx, connect.NewRequest(&posifacev1.SetSaleCustomerRequest{SaleId: saleID, CustomerId: customerID}))
	require.NoError(t, err)
	_, err = svc.AttachPrescription(ctx, connect.NewRequest(&posifacev1.AttachPrescriptionRequest{SaleId: saleID, PrescriptionId: rxID}))
	require.NoError(t, err)
	_, err = svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{SaleId: saleID, ProductId: productID, Qty: 4}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 12000,
	}))
	require.NoError(t, err)

	var dispensed int32
	require.NoError(t, db.Model(&model.PrescriptionItem{}).
		Where("prescription_id = ? AND product_id = ?", rxID, productID).
		Select("dispensed_qty").Scan(&dispensed).Error)
	require.Equal(t, int32(4), dispensed) // bumped by the sale

	_, err = svc.RefundSale(ctx, connect.NewRequest(&posifacev1.RefundSaleRequest{SaleId: saleID, Restock: true}))
	require.NoError(t, err)

	require.NoError(t, db.Model(&model.PrescriptionItem{}).
		Where("prescription_id = ? AND product_id = ?", rxID, productID).
		Select("dispensed_qty").Scan(&dispensed).Error)
	require.Equal(t, int32(0), dispensed) // reversed by the restock refund
}
