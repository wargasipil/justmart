package common

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
)

// Where a lot's expiry date came from — the values of batches.expiry_source.
// The date alone cannot say whether anybody read it off the pack; this can.
const (
	ExpirySourceEntered = "ENTERED" // typed by a person, or confirmed later
	ExpirySourceDefault = "DEFAULT" // the product's default, accepted unchanged
	ExpirySourceNone    = "NONE"    // the goods do not expire; the date is NoExpiryDate
)

// How a product's new lots get their expiry pre-filled — products.expiry_default.
const (
	ExpiryDefaultManual = "MANUAL"
	ExpiryDefaultMonths = "MONTHS"
	ExpiryDefaultNone   = "NONE"
)

// MaxExpiryDefaultMonths bounds products.expiry_default_months. Ten years is
// past any shelf life a shop would pre-fill; anything larger is a typo.
const MaxExpiryDefaultMonths = 120

// NoExpiryDate is the date a lot stores when its goods do not expire. The
// column is NOT NULL, and a far-future date keeps every consumer correct with
// no special case: FEFO sells such lots last and they never count as expiring
// soon. Read expiry_source to tell it apart from a real date.
var NoExpiryDate = time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)

// ExpirySourceFromWire maps the inventory_iface.v1.ExpirySource enum, passed as
// its number, to the stored string. It takes the number so this package stays
// free of generated code while the batch, purchasing and import paths share one
// mapping. UNSPECIFIED (0) means ENTERED: every caller that predates the field
// was sending a date somebody typed. Pinned by TestExpirySourceWireMapping.
func ExpirySourceFromWire(v int32) string {
	switch v {
	case 2:
		return ExpirySourceDefault
	case 3:
		return ExpirySourceNone
	default:
		return ExpirySourceEntered
	}
}

// ExpirySourceToWire is the inverse of ExpirySourceFromWire.
func ExpirySourceToWire(s string) int32 {
	switch s {
	case ExpirySourceDefault:
		return 2
	case ExpirySourceNone:
		return 3
	default:
		return 1
	}
}

// LotExpiry resolves the expiry a lot stores from what a caller sent. NONE
// ignores the date and stores NoExpiryDate; any other source needs a valid
// YYYY-MM-DD. ok is false when that date is missing or malformed.
func LotExpiry(date, source string) (expiry time.Time, ok bool) {
	if source == ExpirySourceNone {
		return NoExpiryDate, true
	}
	t, err := time.Parse(DateLayout, strings.TrimSpace(date))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// NoExpiryAllowed reports whether a lot of this product may be recorded as
// "does not expire". It may, except in pharmacy mode for a product that needs a
// prescription: a medicine always expires, and a lot that claims otherwise is
// sold last by FEFO and never shows up as expiring — the one failure a pharmacy
// cannot afford. Retail is unaffected, as with every pharmacy rule. Callers
// emit their own domain token when this returns false.
func NoExpiryAllowed(ctx context.Context, db *gorm.DB, productID string) (bool, error) {
	pharmacy, err := IsPharmacyMode(ctx, db)
	if err != nil || !pharmacy {
		return true, err
	}
	var p model.Product
	if err := db.WithContext(ctx).Select("prescription_required").
		Where("id = ?", productID).First(&p).Error; err != nil {
		return false, err
	}
	return !p.PrescriptionRequired, nil
}

// ExpiryPassed reports whether a lot's expiry date is before today. Both sides
// are compared as calendar dates: the stored DATE carries no zone, and "today"
// is the shop's local day — the same day the operator sees on the pack.
func ExpiryPassed(expiry, now time.Time) bool {
	return expiry.Format(DateLayout) < now.Format(DateLayout)
}
