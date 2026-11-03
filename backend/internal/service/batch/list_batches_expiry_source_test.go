package batch_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

// The DEFAULT filter is the shelf-check worklist: lots whose expiry is still
// the product's estimate. It must find exactly those, keep an honest total, and
// let a lot drop off the list once somebody confirms its date.
func TestListBatches_FiltersByExpirySource(t *testing.T) {
	t.Parallel()
	e := newExpiryEnv(t)
	estimated := e.lot(t, "LBS-1", "2028-10-31", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_DEFAULT)
	e.lot(t, "LBS-2", "2027-06-30", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	e.lot(t, "LBS-3", "", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_NONE)

	list := func(src inventoryifacev1.ExpirySource) *inventoryifacev1.ListBatchesResponse {
		t.Helper()
		resp, err := e.svc.ListBatches(e.ctx, connect.NewRequest(&inventoryifacev1.ListBatchesRequest{
			ExpirySource: src,
		}))
		require.NoError(t, err)
		return resp.Msg
	}

	require.Equal(t, int32(3), list(inventoryifacev1.ExpirySource_EXPIRY_SOURCE_UNSPECIFIED).Total)

	defaults := list(inventoryifacev1.ExpirySource_EXPIRY_SOURCE_DEFAULT)
	require.Equal(t, int32(1), defaults.Total)
	require.Equal(t, estimated, defaults.Batches[0].Id)

	none := list(inventoryifacev1.ExpirySource_EXPIRY_SOURCE_NONE)
	require.Equal(t, int32(1), none.Total)
	require.Equal(t, "2099-12-31", none.Batches[0].ExpiryDate)

	_, err := e.set(estimated, "2028-10-31", "checked the pack", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	require.NoError(t, err)
	require.Zero(t, list(inventoryifacev1.ExpirySource_EXPIRY_SOURCE_DEFAULT).Total, "a confirmed lot leaves the worklist")
}
