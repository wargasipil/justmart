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
// paid (supplier identity).
//
// Every role may, by owner decision: the catalog is FULLY READABLE by the till.
// A cashier restocking a shelf or answering "why did this go up" needs the same
// figures the manager sees, and splitting the page into two truths cost more
// than the secrecy was worth. Cost visibility is therefore no longer a role
// boundary — WRITING the catalog still is (Create/Update/Archive/Import, the
// price-tier and discount mutations, and every purchasing RPC remain
// OWNER+PHARMACIST via their proto allowed_roles).
//
// This stays a function rather than being deleted so the boundary is one edit
// away if a shop wants it back: the redactors that call it
// (product.redactCost / batch.redactCost) are still wired in as the last step
// of every catalog read, so narrowing this predicate re-hides the fields
// wire-side with no other change. Its frontend mirror is lib/roles.ts
// canSeeCost — change one, change both.
func CanSeeCost(role string) bool {
	switch role {
	case RoleOwner, RolePharmacist, RoleCashier, RoleApoteker:
		return true
	default:
		return false
	}
}
