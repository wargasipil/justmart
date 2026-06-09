package purchasing_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func TestGetSupplierBalances_HappyPath(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-GSB", "Balance supplier")
	prodID := e.seedProduct(t, "gsb-sku", "GSB product", 1000)
	po := e.createPO(t, supID, prodID, 10, 1000) // ordered_total 10000

	// Pay 3000 so outstanding = 7000.
	_, err := e.payments.PayPurchase(e.ctx, connect.NewRequest(&purchasingifacev1.PayPurchaseRequest{
		PurchaseOrderId: po.Id,
		Amount:          3000,
	}))
	require.NoError(t, err)

	resp, err := e.payments.GetSupplierBalances(e.ctx, connect.NewRequest(&purchasingifacev1.GetSupplierBalancesRequest{}))
	require.NoError(t, err)

	var found *purchasingifacev1.SupplierBalance
	for _, b := range resp.Msg.Balances {
		if b.SupplierId == supID {
			found = b
			break
		}
	}
	require.NotNil(t, found, "the seeded supplier should appear in balances")
	require.Equal(t, "Balance supplier", found.SupplierName)
	require.Equal(t, int64(10000), found.OrderedTotal)
	require.Equal(t, int64(3000), found.PaidTotal)
	require.Equal(t, int64(7000), found.Outstanding)
	require.Equal(t, int32(1), found.OpenPoCount) // DRAFT PO counts as open
}

func TestGetSupplierBalances_OnlyOutstandingFilter(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	supID := e.seedSupplier(t, "SUP-GSB2", "No-outstanding supplier")
	prodID := e.seedProduct(t, "gsb2-sku", "GSB2 product", 1000)
	po := e.createPO(t, supID, prodID, 1, 1000) // ordered_total 1000

	// Pay in full so outstanding = 0.
	_, err := e.payments.PayPurchase(e.ctx, connect.NewRequest(&purchasingifacev1.PayPurchaseRequest{
		PurchaseOrderId: po.Id,
		Amount:          1000,
	}))
	require.NoError(t, err)

	// With only_outstanding=true, a fully-paid supplier is excluded.
	resp, err := e.payments.GetSupplierBalances(e.ctx, connect.NewRequest(&purchasingifacev1.GetSupplierBalancesRequest{
		OnlyOutstanding: true,
	}))
	require.NoError(t, err)
	for _, b := range resp.Msg.Balances {
		require.NotEqual(t, supID, b.SupplierId, "fully-paid supplier should be filtered out")
	}
}
