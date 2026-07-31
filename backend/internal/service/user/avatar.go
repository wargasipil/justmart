package user

import (
	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
)

// Payload caps, enforced on both renditions. The client downscales before
// sending, so these are a backstop against a caller that skips that step (or
// lies) — not the expected size. A 512x512 JPEG lands ~40-80 KB; a 128x128
// thumb lands ~4-8 KB.
const (
	MaxAvatarBytes      = 2 << 20   // 2 MiB — the ORIGINAL rendition
	MaxAvatarThumbBytes = 256 << 10 // 256 KiB — the THUMB rendition
)

// allowedAvatarTypes is the set of image types the UI can produce and every
// browser can render. Deliberately excludes SVG: it is script-capable, and we
// hand these bytes back as a blob the browser renders.
var allowedAvatarTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// resolveVariant maps the request enum to a concrete rendition. Unset resolves
// to THUMB so the fast path is what a caller gets by default — shipping the
// original is an explicit opt-in.
func resolveVariant(v userifacev1.AvatarVariant) userifacev1.AvatarVariant {
	if v == userifacev1.AvatarVariant_AVATAR_VARIANT_ORIGINAL {
		return userifacev1.AvatarVariant_AVATAR_VARIANT_ORIGINAL
	}
	return userifacev1.AvatarVariant_AVATAR_VARIANT_THUMB
}
