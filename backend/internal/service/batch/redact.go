package batch

import (
	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/service/common"
)

// redactCost blanks the purchase-side fields of a batch when the caller may not
// see cost (CASHIER / APOTEKER — see common.CanSeeCost). Batch reads are open to
// the till because POS and the read-only Products pages need lot number, expiry
// and quantity; what the lot COST, who supplied it and which PO bought it are
// manager data that happen to sit on the same row.
//
// Runs as the last step of a read handler, after every enrich — same choke-point
// rule as product.redactCost.
func redactCost(caller auth.Principal, batches []*inventoryifacev1.Batch) {
	if common.CanSeeCost(caller.Role) {
		return
	}
	for _, b := range batches {
		b.CostPrice = 0
		b.SupplierId = ""
		b.PurchaseOrderId = ""
		b.PoNo = ""
	}
}
