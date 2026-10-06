package batch_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

// Newest first, paginated with an honest total, and scoped to the one lot.
func TestListBatchExpiryChanges_NewestFirstAndPaged(t *testing.T) {
	t.Parallel()
	e := newExpiryEnv(t)
	id := e.lot(t, "LE-1", "2027-01-31", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	other := e.lot(t, "LE-2", "2027-01-31", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	for _, d := range []string{"2027-02-28", "2027-03-31", "2027-04-30"} {
		_, err := e.set(id, d, "move to "+d, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
		require.NoError(t, err)
	}
	_, err := e.set(other, "2028-01-31", "someone else's lot", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	require.NoError(t, err)

	page := func(limit, offset int32) *inventoryifacev1.ListBatchExpiryChangesResponse {
		t.Helper()
		resp, err := e.svc.ListBatchExpiryChanges(e.ctx, connect.NewRequest(&inventoryifacev1.ListBatchExpiryChangesRequest{
			BatchId: id, Limit: limit, Offset: offset,
		}))
		require.NoError(t, err)
		return resp.Msg
	}

	first := page(2, 0)
	require.Equal(t, int32(3), first.Total)
	require.Len(t, first.Changes, 2)
	require.Equal(t, "2027-04-30", first.Changes[0].NewExpiryDate, "newest first")
	require.Equal(t, "2027-03-31", first.Changes[0].OldExpiryDate)
	require.Equal(t, e.ownerID, first.Changes[0].ChangedBy)

	second := page(2, 2)
	require.Len(t, second.Changes, 1)
	require.Equal(t, "2027-02-28", second.Changes[0].NewExpiryDate)
}

func TestListBatchExpiryChanges_BatchRequired(t *testing.T) {
	t.Parallel()
	e := newExpiryEnv(t)
	_, err := e.svc.ListBatchExpiryChanges(e.ctx, connect.NewRequest(&inventoryifacev1.ListBatchExpiryChangesRequest{}))
	requireBatchToken(t, err, connect.CodeInvalidArgument, "batch.batch_required")
}
