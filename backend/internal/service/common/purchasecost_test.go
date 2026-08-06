package common_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/justmart/backend/internal/service/common"
)

// At rate 0 the helper must reproduce the pre-PPN formula exactly, or every
// existing PO would re-cost itself the moment this shipped.
func TestNetUnitCost_ZeroRateMatchesLegacyFormula(t *testing.T) {
	t.Parallel()
	legacy := func(subtotal, orderedQty, gross int64) int64 {
		if orderedQty <= 0 {
			return gross
		}
		return (subtotal + orderedQty/2) / orderedQty
	}
	// Odd quantities are where a naive rewrite drifts: floor(q/2) vs q/2.
	for _, qty := range []int64{1, 2, 3, 4, 5, 7, 9, 10, 33, 100} {
		for _, subtotal := range []int64{0, 1, 5, 7, 10, 11, 999, 1000, 123457} {
			require.Equal(t, legacy(subtotal, qty, 0), common.NetUnitCost(subtotal, qty, 0, 0),
				"subtotal=%d qty=%d", subtotal, qty)
		}
	}
}

func TestNetUnitCost_CapitalizesPPN(t *testing.T) {
	t.Parallel()
	// 10 base units at 1000 net = 1000/base; +11% => 1110.
	require.Equal(t, int64(1110), common.NetUnitCost(10_000, 10, 0, 11))
	// PPN off on the same line leaves the cost untouched.
	require.Equal(t, int64(1000), common.NetUnitCost(10_000, 10, 0, 0))
	// A non-default rate is honoured, not hardcoded to 11.
	require.Equal(t, int64(1120), common.NetUnitCost(10_000, 10, 0, 12))
}

func TestNetUnitCost_RoundsHalfUpAfterScaling(t *testing.T) {
	t.Parallel()
	// 3 base units at 100 net: 100*1.11 = 111 exactly.
	require.Equal(t, int64(111), common.NetUnitCost(300, 3, 0, 11))
	// 3 base units at 1 net total: 0.333/base, 0.37 with PPN -> rounds to 0.
	require.Equal(t, int64(0), common.NetUnitCost(1, 3, 0, 11))
	// 3 base units at 3 net total: 1/base, 1.11 with PPN -> rounds to 1.
	require.Equal(t, int64(1), common.NetUnitCost(3, 3, 0, 11))
	// Scale-then-round beats round-then-scale: 5/2 = 2.5 -> would round to 3
	// then scale to 333; scaling first gives 5*111/200 = 2.775 -> 3. Pin the
	// value so the order of operations can't silently flip.
	require.Equal(t, int64(3), common.NetUnitCost(5, 2, 0, 11))
	// Exact .5 rounds up.
	require.Equal(t, int64(3), common.NetUnitCost(5, 2, 0, 0))
}

func TestNetUnitCost_FallsBackToGrossWhenNoQty(t *testing.T) {
	t.Parallel()
	require.Equal(t, int64(500), common.NetUnitCost(0, 0, 500, 0))
	// The fallback is scaled too — PPN must not depend on which branch ran.
	require.Equal(t, int64(555), common.NetUnitCost(0, 0, 500, 11))
	require.Equal(t, int64(555), common.NetUnitCost(0, -3, 500, 11))
}

func TestNetUnitCost_ClampsRate(t *testing.T) {
	t.Parallel()
	require.Equal(t, int64(1000), common.NetUnitCost(10_000, 10, 0, -5))
	require.Equal(t, int64(2000), common.NetUnitCost(10_000, 10, 0, 250))
}
