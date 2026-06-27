package priceagreement_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestGetPriceAgreement_RoundTrip(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "GET-A")
	prod, _, box := seedProductWithUnits(t, db, "get-1")

	created, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
		SupplierId: sup, ProductId: prod, ProductUnitId: box, Price: 5000,
	}))
	require.NoError(t, err)

	got, err := svc.GetPriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.GetPriceAgreementRequest{Id: created.Msg.Agreement.Id}))
	require.NoError(t, err)
	require.Equal(t, created.Msg.Agreement.Id, got.Msg.Agreement.Id)
	require.Equal(t, int64(5000), got.Msg.Agreement.Price)
}

func TestGetPriceAgreement_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newSvc(t)
	_, err := svc.GetPriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.GetPriceAgreementRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
