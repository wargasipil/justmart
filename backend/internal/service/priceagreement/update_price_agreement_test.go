package priceagreement_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestUpdatePriceAgreement_ChangesPriceAndUnit(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "UP-A")
	prod, base, box := seedProductWithUnits(t, db, "up-1")

	created, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
		SupplierId: sup, ProductId: prod, ProductUnitId: box, Price: 140000,
	}))
	require.NoError(t, err)

	upd, err := svc.UpdatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.UpdatePriceAgreementRequest{
		Id: created.Msg.Agreement.Id, ProductUnitId: base, Price: 12000, Note: "switch to pcs",
	}))
	require.NoError(t, err)
	require.Equal(t, base, upd.Msg.Agreement.ProductUnitId)
	require.Equal(t, "pcs", upd.Msg.Agreement.UnitName)   // re-snapshotted
	require.Equal(t, int64(1), upd.Msg.Agreement.UnitFactor)
	require.Equal(t, int64(12000), upd.Msg.Agreement.Price)
	require.Equal(t, "switch to pcs", upd.Msg.Agreement.Note)
}

func TestUpdatePriceAgreement_UnitClashRejected(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "UP-B")
	prod, base, box := seedProductWithUnits(t, db, "up-2")

	// Two agreements: one on box, one on pcs.
	_, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
		SupplierId: sup, ProductId: prod, ProductUnitId: box, Price: 140000,
	}))
	require.NoError(t, err)
	pcsAgreement, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
		SupplierId: sup, ProductId: prod, ProductUnitId: base, Price: 12000,
	}))
	require.NoError(t, err)

	// Moving the pcs one to box collides with the existing box agreement.
	_, err = svc.UpdatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.UpdatePriceAgreementRequest{
		Id: pcsAgreement.Msg.Agreement.Id, ProductUnitId: box, Price: 999,
	}))
	requireToken(t, err, connect.CodeAlreadyExists, "price_agreement.exists")
}
