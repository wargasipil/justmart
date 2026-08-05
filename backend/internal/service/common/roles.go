package common

// Role strings as stored in users.role (the stripped proto enum name — see
// auth.roleEnumToString). Handlers compare caller.Role against these.
const (
	RoleOwner      = "OWNER"
	RolePharmacist = "PHARMACIST"
	RoleCashier    = "CASHIER"
	RoleApoteker   = "APOTEKER"
)

// CanSeeCost reports whether a caller may receive purchase-cost data: what the
// shop PAID (batch cost, restock price, valuation, reference cost) and who it
// paid (supplier identity). That is manager information — the same posture as
// GetProductsSummary being narrower than ListProducts and GetMyPerformance
// omitting COGS.
//
// The till roles (CASHIER / APOTEKER) legitimately read the catalog: POS needs
// it, and the read-only Products pages are open to them. So the cost fields ride
// along on responses they are allowed to fetch, and hiding them in the UI alone
// would leave them in the response body. Handlers therefore blank them BEFORE
// responding — see product.redactCost / batch.redactCost.
func CanSeeCost(role string) bool {
	return role == RoleOwner || role == RolePharmacist
}
