package priceagreement_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

// Archiving frees the (supplier, product, unit) slot so a new active agreement
// can be created for the same triple.
func TestArchivePriceAgreement_FreesSlot(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "AR-A")
	prod, _, box := seedProductWithUnits(t, db, "ar-1")

	mk := func() (string, error) {
		r, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
			SupplierId: sup, ProductId: prod, ProductUnitId: box, Price: 1000,
		}))
		if err != nil {
			return "", err
		}
		return r.Msg.Agreement.Id, nil
	}

	id, err := mk()
	require.NoError(t, err)

	// Duplicate blocked while active.
	_, err = mk()
	requireToken(t, err, connect.CodeAlreadyExists, "price_agreement.exists")

	// Archive → slot freed.
	arch, err := svc.ArchivePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.ArchivePriceAgreementRequest{Id: id}))
	require.NoError(t, err)
	require.False(t, arch.Msg.Agreement.Active)

	// Now a new active agreement for the same triple succeeds.
	_, err = mk()
	require.NoError(t, err)
}
