package common

import (
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"
)

// AsConnectErr passes through connect errors and wraps others as Internal.
func AsConnectErr(err error) error {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return err
	}
	return connect.NewError(connect.CodeInternal, err)
}

// TokenError returns a connect error whose Message is a STABLE machine token
// (dotted "<domain>.<reason>"), never English prose. The frontend maps the token
// to a translated, field-attributed message (see frontend/src/lib/serverErrors.ts).
//
// TOKEN CATALOG MIRROR — every token returned here must be kept in sync with the
// frontend serverErrors.ts map (tokens absent there fall back to a generic
// translated toast, never leaking the raw token). Use for every field-attributable
// validation / uniqueness failure.
func TokenError(code connect.Code, token string) error {
	return connect.NewError(code, errors.New(token))
}

// ExistsBy reports whether any row of the given model matches the where clause.
// Used for pre-insert/update uniqueness checks so a specific "*_taken" token can
// be returned instead of decoding a raw, engine-specific driver constraint error.
func ExistsBy(db *gorm.DB, model any, query string, args ...any) (bool, error) {
	var n int64
	if err := db.Model(model).Where(query, args...).Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// Deref returns the pointed-to string, or "" if the pointer is nil.
func Deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
