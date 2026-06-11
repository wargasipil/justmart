package license_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/justmart/backend/internal/license"
	"github.com/justmart/backend/security"
)

func sign(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(security.SecretRoot))
	require.NoError(t, err)
	return s
}

func TestVerify_Valid(t *testing.T) {
	t.Parallel()
	tok := sign(t, jwt.MapClaims{
		"name":             "Apotek Sehat",
		"business_type":    "BUSSINESS_TYPE_PHARMACY_SHOP",
		"business_type_id": 1,
		"iat":              time.Now().Unix(),
	})
	c, err := license.Verify(tok)
	require.NoError(t, err)
	require.Equal(t, "Apotek Sehat", c.Name)
	require.Equal(t, int32(1), c.BusinessType)
}

func TestVerify_WrongKey(t *testing.T) {
	t.Parallel()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"business_type_id": 1})
	s, err := tok.SignedString([]byte("not-the-real-secret"))
	require.NoError(t, err)
	_, err = license.Verify(s)
	require.Error(t, err)
}

func TestVerify_Expired(t *testing.T) {
	t.Parallel()
	tok := sign(t, jwt.MapClaims{"business_type_id": 2, "exp": time.Now().Add(-time.Hour).Unix()})
	_, err := license.Verify(tok)
	require.Error(t, err)
}

func TestVerify_Empty(t *testing.T) {
	t.Parallel()
	_, err := license.Verify("")
	require.Error(t, err)
}
