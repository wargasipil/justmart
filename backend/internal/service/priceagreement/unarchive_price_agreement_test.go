package priceagreement_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestUnarchivePriceAgreement_RoundTrip(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "UA-A")
	prod, _, box := seedProductWithUnits(t, db, "ua-1")

	created, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
		SupplierId: sup, ProductId: prod, ProductUnitId: box, Price: 1000,
	}))
	require.NoError(t, err)
	id := created.Msg.Agreement.Id

	_, err = svc.ArchivePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.ArchivePriceAgreementRequest{Id: id}))
	require.NoError(t, err)

	resp, err := svc.UnarchivePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.UnarchivePriceAgreementRequest{Id: id}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Agreement.Active)

	// It's back in the default (active-only) list.
	def, err := svc.ListPriceAgreements(ctx(), connect.NewRequest(&inventoryifacev1.ListPriceAgreementsRequest{SupplierId: sup}))
	require.NoError(t, err)
	require.Equal(t, int32(1), def.Msg.Total)
}

func TestUnarchivePriceAgreement_BlockedByActiveDuplicate(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "UA-B")
	prod, _, box := seedProductWithUnits(t, db, "ua-2")

	// Create + archive the original, then create a new active one for the same
	// (supplier, product, unit) — unarchiving the original must collide.
	first, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
		SupplierId: sup, ProductId: prod, ProductUnitId: box, Price: 1000,
	}))
	require.NoError(t, err)
	_, err = svc.ArchivePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.ArchivePriceAgreementRequest{Id: first.Msg.Agreement.Id}))
	require.NoError(t, err)
	_, err = svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
		SupplierId: sup, ProductId: prod, ProductUnitId: box, Price: 2000,
	}))
	require.NoError(t, err)

	_, err = svc.UnarchivePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.UnarchivePriceAgreementRequest{Id: first.Msg.Agreement.Id}))
	requireToken(t, err, connect.CodeAlreadyExists, "price_agreement.exists")
}
