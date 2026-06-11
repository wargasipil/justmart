package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/license"
)

// A minted license must verify with the in-binary key and carry the right name +
// business type — closing the generator <-> verifier loop in code.
func TestBuildLicenseToken_RoundTrip(t *testing.T) {
	t.Parallel()
	tok, err := buildLicenseToken("Apotek Sehat", settingsifacev1.BussinessType_BUSSINESS_TYPE_PHARMACY_SHOP, 0)
	require.NoError(t, err)

	c, err := license.Verify(tok)
	require.NoError(t, err)
	require.Equal(t, "Apotek Sehat", c.Name)
	require.Equal(t, int32(settingsifacev1.BussinessType_BUSSINESS_TYPE_PHARMACY_SHOP), c.BusinessType)
}

func TestHumanizeBusinessType(t *testing.T) {
	t.Parallel()
	require.Equal(t, "Pharmacy shop", humanizeBusinessType("BUSSINESS_TYPE_PHARMACY_SHOP"))
	require.Equal(t, "Retail", humanizeBusinessType("BUSSINESS_TYPE_RETAIL"))
}
