package priceagreement_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestCreatePriceAgreement_RoundTrip(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "SUP-A")
	prod, _, box := seedProductWithUnits(t, db, "pa-1")

	resp, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
		SupplierId: sup, ProductId: prod, ProductUnitId: box,
		Price: 140000, ValidFrom: "2026-01-01", ValidUntil: "2026-12-31", Note: "kontrak 2026",
	}))
	require.NoError(t, err)
	a := resp.Msg.Agreement
	require.NotEmpty(t, a.Id)
	require.Equal(t, sup, a.SupplierId)
	require.Equal(t, prod, a.ProductId)
	require.Equal(t, box, a.ProductUnitId)
	require.Equal(t, "box", a.UnitName)        // snapshotted
	require.Equal(t, int64(12), a.UnitFactor)  // snapshotted
	require.Equal(t, int64(140000), a.Price)
	require.Equal(t, "2026-01-01", a.ValidFrom)
	require.Equal(t, "2026-12-31", a.ValidUntil)
	require.True(t, a.Active)
}

func TestCreatePriceAgreement_DuplicateActiveRejected(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "SUP-B")
	prod, _, box := seedProductWithUnits(t, db, "pa-2")

	mk := func() error {
		_, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
			SupplierId: sup, ProductId: prod, ProductUnitId: box, Price: 1000,
		}))
		return err
	}
	require.NoError(t, mk())
	requireToken(t, mk(), connect.CodeAlreadyExists, "price_agreement.exists")
}

func TestCreatePriceAgreement_MissingSupplier(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	prod, _, box := seedProductWithUnits(t, db, "pa-3")

	_, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
		SupplierId: "00000000-0000-0000-0000-000000000000", ProductId: prod, ProductUnitId: box, Price: 1000,
	}))
	requireToken(t, err, connect.CodeFailedPrecondition, "price_agreement.supplier_missing")
}

func TestCreatePriceAgreement_UnitMustBelongToProduct(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "SUP-C")
	prod, _, _ := seedProductWithUnits(t, db, "pa-4")
	_, _, otherBox := seedProductWithUnits(t, db, "pa-4b") // a different product's unit

	_, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
		SupplierId: sup, ProductId: prod, ProductUnitId: otherBox, Price: 1000,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestCreatePriceAgreement_BadDates(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "SUP-D")
	prod, _, box := seedProductWithUnits(t, db, "pa-5")

	_, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
		SupplierId: sup, ProductId: prod, ProductUnitId: box, Price: 1000,
		ValidFrom: "2026-12-31", ValidUntil: "2026-01-01",
	}))
	requireToken(t, err, connect.CodeInvalidArgument, "price_agreement.bad_dates")
}
