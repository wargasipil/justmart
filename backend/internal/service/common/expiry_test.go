package common_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// ExpirySourceFromWire takes the enum as a bare number so common stays free of
// generated code. This pins that number to the generated constant, so a
// renumbered proto enum breaks here instead of silently mislabelling lots.
func TestExpirySourceWireMapping(t *testing.T) {
	cases := map[inventoryifacev1.ExpirySource]string{
		inventoryifacev1.ExpirySource_EXPIRY_SOURCE_UNSPECIFIED: common.ExpirySourceEntered,
		inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED:     common.ExpirySourceEntered,
		inventoryifacev1.ExpirySource_EXPIRY_SOURCE_DEFAULT:     common.ExpirySourceDefault,
		inventoryifacev1.ExpirySource_EXPIRY_SOURCE_NONE:        common.ExpirySourceNone,
	}
	for wire, stored := range cases {
		require.Equal(t, stored, common.ExpirySourceFromWire(int32(wire)), wire.String())
		if wire != inventoryifacev1.ExpirySource_EXPIRY_SOURCE_UNSPECIFIED {
			require.Equal(t, int32(wire), common.ExpirySourceToWire(stored), wire.String())
		}
	}
}

func TestLotExpiry(t *testing.T) {
	got, ok := common.LotExpiry(" 2027-03-31 ", common.ExpirySourceEntered)
	require.True(t, ok)
	require.Equal(t, "2027-03-31", got.Format(common.DateLayout))

	_, ok = common.LotExpiry("", common.ExpirySourceDefault)
	require.False(t, ok, "a dated source needs a date")

	got, ok = common.LotExpiry("", common.ExpirySourceNone)
	require.True(t, ok, "NONE ignores the date")
	require.Equal(t, common.NoExpiryDate, got)
}

// Calendar dates on both sides: a lot expiring today has not passed.
func TestExpiryPassed(t *testing.T) {
	now := time.Date(2026, 10, 6, 23, 30, 0, 0, time.Local)
	day := func(s string) time.Time {
		d, err := time.Parse(common.DateLayout, s)
		require.NoError(t, err)
		return d
	}
	require.True(t, common.ExpiryPassed(day("2026-10-05"), now))
	require.False(t, common.ExpiryPassed(day("2026-10-06"), now))
	require.False(t, common.ExpiryPassed(day("2026-10-07"), now))
}
