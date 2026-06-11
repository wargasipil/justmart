// Package license verifies offline license tokens minted by cmd/license. A
// license is a JWT (HS256) signed with the in-binary key security.SecretRoot;
// it carries the licensed holder name + business type, which drives the app's
// business mode (retail vs pharmacy).
package license

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/justmart/backend/security"
)

// Claims is the validated payload of a license token.
type Claims struct {
	Name         string // licensed holder / business name
	BusinessType int32  // settings_iface.v1.BussinessType enum value
}

// Issue mints a license token (HS256, signed with security.SecretRoot) carrying
// the holder name + business type, with an optional TTL (0 = perpetual). It is
// the programmatic counterpart of the cmd/license CLI — used by tests and any
// in-process minting. Verify round-trips the result.
func Issue(name string, businessType int32, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"name":             name,
		"business_type_id": businessType,
		"iat":              now.Unix(),
	}
	if ttl > 0 {
		claims["exp"] = now.Add(ttl).Unix()
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(security.SecretRoot))
}

// Verify parses and verifies a license token against security.SecretRoot,
// enforcing the HS256 signing method and any expiry. Returns the claims, or an
// error for an empty / malformed / wrong-key / expired token.
func Verify(token string) (Claims, error) {
	if token == "" {
		return Claims{}, errors.New("empty license")
	}
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(security.SecretRoot), nil
	})
	if err != nil {
		return Claims{}, err
	}
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return Claims{}, errors.New("invalid license")
	}
	var c Claims
	if v, ok := mc["name"].(string); ok {
		c.Name = v
	}
	// JSON numbers decode to float64.
	if v, ok := mc["business_type_id"].(float64); ok {
		c.BusinessType = int32(v)
	}
	return c, nil
}
