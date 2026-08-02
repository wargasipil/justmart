package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/justmart/backend/gen/pos_iface/v1"   // register SaleService descriptors
	_ "github.com/justmart/backend/gen/table_iface/v1" // register TableService descriptors
	"github.com/justmart/backend/internal/auth"
)

// The WAITER role exists for ONE reason: a floor waiter builds the order, and
// only the till settles it. That split lives entirely in proto `allowed_roles`
// annotations, which are easy to extend by reflex — someone adding a role to a
// block of RPCs, or copying a CASHIER grant set wholesale, would silently hand
// waiters the till and nothing would fail.
//
// BuildPolicy is the single place those annotations become enforcement, so this
// asserts the intended shape against it directly. If a grant here is ever
// changed deliberately, change this test in the same commit — that is the point.
func TestBuildPolicy_WaiterBuildsOrdersButCannotSettleThem(t *testing.T) {
	policy := auth.BuildPolicy()

	allows := func(t *testing.T, procedure string) bool {
		t.Helper()
		p, ok := policy[procedure]
		require.True(t, ok, "procedure %s not found in the policy map", procedure)
		_, allowed := p.AllowedRoles["WAITER"]
		return allowed
	}

	// The cart-building half — a waiter must have all of it.
	for _, proc := range []string{
		"/pos_iface.v1.SaleService/StartSale",
		"/pos_iface.v1.SaleService/GetSale",
		"/pos_iface.v1.SaleService/AddItem",
		"/pos_iface.v1.SaleService/SetItemQuantity",
		"/pos_iface.v1.SaleService/RemoveItem",
		"/pos_iface.v1.SaleService/SetSaleCustomer",
		"/pos_iface.v1.SaleService/FireToKitchen",
		"/pos_iface.v1.SaleService/SetItemNote",
		"/pos_iface.v1.SaleService/DiscardSale",
		"/table_iface.v1.TableService/ListTables",
		"/table_iface.v1.TableService/OpenTable",
		"/table_iface.v1.TableService/MoveOrder",
	} {
		require.True(t, allows(t, proc), "WAITER must be able to call %s", proc)
	}

	// The till half — a waiter must have NONE of it. Money and stock reversal
	// stay with the cashier tier; discounts and the service fee are pricing
	// decisions, not floor service.
	for _, proc := range []string{
		"/pos_iface.v1.SaleService/CompleteSale",
		"/pos_iface.v1.SaleService/VoidSale",
		"/pos_iface.v1.SaleService/RefundSale",
		"/pos_iface.v1.SaleService/PrintReceipt",
		"/pos_iface.v1.SaleService/SetLineDiscount",
		"/pos_iface.v1.SaleService/ClearLineDiscount",
		"/pos_iface.v1.SaleService/SetCartDiscount",
		"/pos_iface.v1.SaleService/SetServiceFee",
	} {
		require.False(t, allows(t, proc), "WAITER must NOT be able to call %s", proc)
	}

	// Table ADMINISTRATION is manager tier — a waiter serves the floor, they
	// don't redraw it.
	for _, proc := range []string{
		"/table_iface.v1.TableService/CreateTable",
		"/table_iface.v1.TableService/UpdateTable",
		"/table_iface.v1.TableService/ArchiveTable",
		"/table_iface.v1.TableService/UnarchiveTable",
	} {
		require.False(t, allows(t, proc), "WAITER must NOT be able to call %s", proc)
	}
}

// A role set is only meaningful if the shared roles kept their access — a
// regression that dropped CASHIER while adding WAITER would pass the test above.
func TestBuildPolicy_CashierKeepsTillAuthority(t *testing.T) {
	policy := auth.BuildPolicy()
	for _, proc := range []string{
		"/pos_iface.v1.SaleService/CompleteSale",
		"/pos_iface.v1.SaleService/VoidSale",
		"/pos_iface.v1.SaleService/PrintReceipt",
	} {
		p, ok := policy[proc]
		require.True(t, ok, "procedure %s not found", proc)
		_, allowed := p.AllowedRoles["CASHIER"]
		require.True(t, allowed, "CASHIER must still be able to call %s", proc)
	}
}
