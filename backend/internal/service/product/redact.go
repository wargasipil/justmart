package product

import (
	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/service/common"
)

// redactCost blanks every cost-bearing field on a page of products when the
// caller may not see cost (CASHIER / APOTEKER — see common.CanSeeCost). It runs
// as the LAST step of a read handler, after every enrich, so a new enrichment
// cannot leak by forgetting a branch: adding a cost field means adding one line
// here, not auditing each call site.
//
// Sell-side data (unit_price, units, price_tiers) is deliberately untouched —
// a cashier sells at those prices and POS renders them.
func redactCost(caller auth.Principal, products []*inventoryifacev1.Product) {
	if common.CanSeeCost(caller.Role) {
		return
	}
	for _, p := range products {
		// Valuations + the markup/margin reference.
		p.StockValuation = 0
		p.OnOrderValuation = 0
		p.ReferenceCost = 0
		// The whole last-restock block: price and discount ARE cost; qty, dates
		// and the supplier describe the purchase that set it, and the till has
		// no use for them.
		p.LastRestockPrice = 0
		p.LastRestockQty = 0
		p.LastRestockDiscountType = ""
		p.LastRestockDiscountValue = 0
		p.LastRestockCreatedAt = 0
		p.LastRestockArrivedAt = 0
		p.LastRestockSupplierId = ""
		// GetProduct's denormalized pair of the same thing.
		p.LastRestockDate = ""
		p.LastRestockSupplier = ""
	}
}
