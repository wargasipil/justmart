package common

// Order types — the bare strings stored in sales.order_type, mirroring
// table_iface.v1.OrderType. Shared here because two domains write them: `table`
// (OpenTable always writes DINE_IN) and `sale` (StartSale accepts the two
// counter types).
//
// The empty string is the fourth, unnamed state: "not a restaurant flow". Every
// pre-existing sale and every retail/pharmacy sale forever carries it, which is
// why the column defaults to '' rather than to a real type.
const (
	OrderTypeDineIn   = "DINE_IN"
	OrderTypeTakeaway = "TAKEAWAY"
	OrderTypeDelivery = "DELIVERY"
)

// IsValidOrderType reports whether s may be stored. "" is valid — it is the
// non-restaurant default, not a missing value.
func IsValidOrderType(s string) bool {
	switch s {
	case "", OrderTypeDineIn, OrderTypeTakeaway, OrderTypeDelivery:
		return true
	default:
		return false
	}
}

// IsCounterOrderType reports whether s is one a caller may set when STARTING an
// ordinary sale. DINE_IN is excluded on purpose: seating an order is what
// TableService.OpenTable does, and it is the only path that binds a table and
// takes the one-open-bill-per-table lock. Letting StartSale claim DINE_IN would
// produce a dine-in sale attached to no table.
func IsCounterOrderType(s string) bool {
	switch s {
	case "", OrderTypeTakeaway, OrderTypeDelivery:
		return true
	default:
		return false
	}
}
