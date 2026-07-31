package product

import (
	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

// Payload caps, enforced on both renditions. The client downscales before
// sending, so these are a backstop against a caller that skips that step (or
// lies) — not the expected size. A 1024px-bounded JPEG lands ~80-250 KB; a
// 128x128 thumb lands ~4-8 KB.
const (
	MaxProductImageBytes      = 2 << 20   // 2 MiB — the ORIGINAL rendition
	MaxProductImageThumbBytes = 256 << 10 // 256 KiB — the THUMB rendition
)

// allowedProductImageTypes is the set of image types the UI can produce and
// every browser can render. Deliberately excludes SVG: it is script-capable,
// and we hand these bytes back as a blob the browser renders.
var allowedProductImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// resolveImageVariant maps the request enum to a concrete rendition. Unset
// resolves to THUMB so the fast path is what a caller gets by default —
// shipping the original is an explicit opt-in.
func resolveImageVariant(v inventoryifacev1.ProductImageVariant) inventoryifacev1.ProductImageVariant {
	if v == inventoryifacev1.ProductImageVariant_PRODUCT_IMAGE_VARIANT_ORIGINAL {
		return inventoryifacev1.ProductImageVariant_PRODUCT_IMAGE_VARIANT_ORIGINAL
	}
	return inventoryifacev1.ProductImageVariant_PRODUCT_IMAGE_VARIANT_THUMB
}
