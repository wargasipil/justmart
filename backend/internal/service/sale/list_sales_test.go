package sale_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	salesvc "github.com/justmart/backend/internal/service/sale"
	"github.com/justmart/backend/internal/service/servicetest"
)

// completeOne adds qty of productID to the draft saleID and completes it (cash).
func completeOne(t *testing.T, svc *salesvc.SaleService, ctx context.Context, productID, saleID string, qty int32, paid int64) {
	t.Helper()
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: productID, Qty: qty,
	}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: paid,
	}))
	require.NoError(t, err)
}

func TestListSales_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	productID := seedProduct(t, db, "ls-sku-1", "Paracetamol", 2000)
	seedStock(t, db, productID, ownerID, 50)
	saleID := startDraft(t, svc, ctx)
	completeOne(t, svc, ctx, productID, saleID, 2, 10000)

	resp, err := svc.ListSales(ctx, connect.NewRequest(&posifacev1.ListSalesRequest{
		Limit: 100,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Len(t, resp.Msg.Sales, 1)
	require.Equal(t, saleID, resp.Msg.Sales[0].Id)
	// enrichSaleNames denormalizes the product name onto the item.
	require.Equal(t, "Paracetamol", resp.Msg.Sales[0].Items[0].ProductName)
}

func TestListSales_ExcludesDraft(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)
	// A lone DRAFT cart must NOT appear in the order-history list.
	_ = startDraft(t, svc, ctx)

	resp, err := svc.ListSales(ctx, connect.NewRequest(&posifacev1.ListSalesRequest{Limit: 100}))
	require.NoError(t, err)
	require.Equal(t, int32(0), resp.Msg.Total)
	require.Empty(t, resp.Msg.Sales)
}

func TestListSales_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newSaleSvc(t)

	_, err := svc.ListSales(context.Background(), connect.NewRequest(&posifacev1.ListSalesRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

// seedOwnerAndCashierSales seeds one product + stock, then completes one sale as
// the owner and one as a freshly-seeded cashier. Returns the cashier id.
func seedOwnerAndCashierSales(t *testing.T, svc *salesvc.SaleService, ownerCtx context.Context, db *gorm.DB, ownerID string) (cashierID string, cashierCtx context.Context) {
	t.Helper()
	productID := seedProduct(t, db, "ls-scope-sku", "Paracetamol", 2000)
	seedStock(t, db, productID, ownerID, 100)
	// Owner sale: 2 units.
	ownerSale := startDraft(t, svc, ownerCtx)
	completeOne(t, svc, ownerCtx, productID, ownerSale, 2, 10000)
	// Cashier sale: 3 units.
	cashierID = seedCashier(t, db, "ls-cashier@x.test")
	cashierCtx = servicetest.CtxAs(context.Background(), "CASHIER", cashierID)
	cashierSale := startDraft(t, svc, cashierCtx)
	completeOne(t, svc, cashierCtx, productID, cashierSale, 3, 10000)
	return cashierID, cashierCtx
}

func TestListSales_CashierSeesOnlyOwn(t *testing.T) {
	t.Parallel()
	svc, ownerCtx, db, ownerID := newSaleSvc(t)
	cashierID, cashierCtx := seedOwnerAndCashierSales(t, svc, ownerCtx, db, ownerID)

	// Owner sees both sales.
	ownerResp, err := svc.ListSales(ownerCtx, connect.NewRequest(&posifacev1.ListSalesRequest{Limit: 100}))
	require.NoError(t, err)
	require.Equal(t, int32(2), ownerResp.Msg.Total)

	// Cashier sees only their own.
	cashierResp, err := svc.ListSales(cashierCtx, connect.NewRequest(&posifacev1.ListSalesRequest{Limit: 100}))
	require.NoError(t, err)
	require.Equal(t, int32(1), cashierResp.Msg.Total)
	require.Len(t, cashierResp.Msg.Sales, 1)
	require.Equal(t, cashierID, cashierResp.Msg.Sales[0].CashierUserId)
}

func TestListSales_OwnerFilterByCashier(t *testing.T) {
	t.Parallel()
	svc, ownerCtx, db, ownerID := newSaleSvc(t)
	cashierID, _ := seedOwnerAndCashierSales(t, svc, ownerCtx, db, ownerID)

	// Owner filtering to the cashier sees only the cashier's sale.
	resp, err := svc.ListSales(ownerCtx, connect.NewRequest(&posifacev1.ListSalesRequest{
		Limit: 100, CashierUserId: cashierID,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Equal(t, cashierID, resp.Msg.Sales[0].CashierUserId)
}

func TestListSales_CashierForeignIdRejected(t *testing.T) {
	t.Parallel()
	svc, ownerCtx, db, ownerID := newSaleSvc(t)
	_, cashierCtx := seedOwnerAndCashierSales(t, svc, ownerCtx, db, ownerID)

	// A cashier may not request another user's id.
	_, err := svc.ListSales(cashierCtx, connect.NewRequest(&posifacev1.ListSalesRequest{
		Limit: 100, CashierUserId: ownerID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
