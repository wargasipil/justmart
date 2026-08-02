package common

// Product kinds — the bare strings stored in products.product_kind (mirroring
// how users.role stores its enum, so rows stay readable in psql). Shared here
// because three domains read them: `product` validates writes, `productrecipe`
// requires a COMPOSITE parent with STOCKED components, and `sale` branches on
// them to decide what a completed line consumes.
const (
	// ProductKindStocked: the product IS the stock — a sale FEFO-consumes its
	// own batches. The default, and what every pre-kind row reads as.
	ProductKindStocked = "STOCKED"
	// ProductKindComposite: assembled at sale time — a sale consumes the
	// components on its recipe (product_recipe_items) instead of its own batches.
	ProductKindComposite = "COMPOSITE"
	// ProductKindService: consumes no stock at all (corkage, delivery fee,
	// plating charge). Prices and sells like any other product.
	ProductKindService = "SERVICE"
)

// NormalizeProductKind maps the stored value to a kind, treating "" (and any
// unrecognized value) as STOCKED. Reading rather than writing is where the
// fallback belongs: it keeps every pre-migration row and every caller that never
// sets a kind on exactly the original behaviour.
func NormalizeProductKind(kind string) string {
	switch kind {
	case ProductKindComposite:
		return ProductKindComposite
	case ProductKindService:
		return ProductKindService
	default:
		return ProductKindStocked
	}
}

// IsValidProductKind reports whether a kind may be WRITTEN. Note "" is not
// valid here even though NormalizeProductKind reads it as STOCKED — a write
// path should store the explicit value, and callers map UNSPECIFIED to STOCKED
// before validating.
func IsValidProductKind(kind string) bool {
	switch kind {
	case ProductKindStocked, ProductKindComposite, ProductKindService:
		return true
	default:
		return false
	}
}
