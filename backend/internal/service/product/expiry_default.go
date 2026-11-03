package product

import (
	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// expiryDefaultFromProto validates a product's expiry setting off the wire and
// returns what the row stores. Shared by CreateProduct (and so ImportProducts)
// and UpdateProduct, so both refuse the same inputs. Months are kept only for
// MONTHS: a stray number beside MANUAL would otherwise sit in the row looking
// like a setting nobody chose.
func expiryDefaultFromProto(d inventoryifacev1.ExpiryDefault, months int32) (string, int32, error) {
	switch d {
	case inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_UNSPECIFIED,
		inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_MANUAL:
		return common.ExpiryDefaultManual, 0, nil
	case inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_MONTHS:
		if months < 1 || months > common.MaxExpiryDefaultMonths {
			return "", 0, common.TokenError(connect.CodeInvalidArgument, "product.expiry_months_invalid")
		}
		return common.ExpiryDefaultMonths, months, nil
	case inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_NONE:
		return common.ExpiryDefaultNone, 0, nil
	default:
		return "", 0, common.TokenError(connect.CodeInvalidArgument, "product.expiry_default_invalid")
	}
}

// expiryDefaultToProto maps the stored string back. Anything unrecognised
// reads as MANUAL, the behaviour every product had before the setting existed.
func expiryDefaultToProto(s string) inventoryifacev1.ExpiryDefault {
	switch s {
	case common.ExpiryDefaultMonths:
		return inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_MONTHS
	case common.ExpiryDefaultNone:
		return inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_NONE
	default:
		return inventoryifacev1.ExpiryDefault_EXPIRY_DEFAULT_MANUAL
	}
}
