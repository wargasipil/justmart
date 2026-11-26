package purchasing_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

// Returning whole boxes credits the lot's exact pack cost: 2 boxes of 30
// bought at Rp 65.000 credit 130.000, not 2 x 30 x 2.167 = 130.020. Same
// rounding the order total had; the supplier debt must not drift either.
func TestCreatePurchaseReturn_WholePacksCreditExactPackCost(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	po := e.createPackPO(t, "RET-PACK", false, packLine)
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 50, "RET-PACK-B1")
	itemID, _ := e.receiptItem(t, rcpt)

	ret, err := e.doReturn(po.Id, itemID, 2, "rusak")
	require.NoError(t, err)
	require.Equal(t, int64(130_000), ret.RefundAmount)

	got := e.getPO(t, po.Id)
	require.Equal(t, int64(130_000), got.ReturnedAmount)
	require.Equal(t, int64(3_250_000-130_000), got.Outstanding)
}

// A lot with no exact pack cost (a line saved before 00064) keeps the
// per-base credit, exactly as before.
func TestCreatePurchaseReturn_LegacyLotKeepsPerBaseCredit(t *testing.T) {
	t.Parallel()
	e := newPOEnv(t)
	po := e.createPackPO(t, "RET-PACK-LEG", false, func(prodID, boxID string) *purchasingifacev1.PurchaseOrderItemInput {
		in := packLine(prodID, boxID)
		in.PackCostPrice = 0
		return in
	})
	rcpt := e.receiveFull(t, po.Id, po.Items[0].Id, 50, "RET-PACK-LEG-B1")
	itemID, _ := e.receiptItem(t, rcpt)

	ret, err := e.doReturn(po.Id, itemID, 2, "rusak")
	require.NoError(t, err)
	require.Equal(t, int64(2*30*2_167), ret.RefundAmount)
}
