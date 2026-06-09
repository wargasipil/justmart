package supplier_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/service/servicetest"
	suppliersvc "github.com/justmart/backend/internal/service/supplier"
)

func TestResolveSuppliers_ReturnsRefs(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	a := seedSupplier(t, svc, "RS-1", "Bernofarm")
	b := seedSupplier(t, svc, "RS-2", "Combiphar")

	resp, err := svc.ResolveSuppliers(context.Background(), connect.NewRequest(&inventoryifacev1.ResolveSuppliersRequest{
		Ids: []string{a.Id, b.Id, "00000000-0000-0000-0000-000000000000"}, // unknown id omitted
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Suppliers, 2)

	byID := map[string]*inventoryifacev1.SupplierRef{}
	for _, r := range resp.Msg.Suppliers {
		byID[r.Id] = r
	}
	require.Equal(t, "Bernofarm", byID[a.Id].Name)
	require.Equal(t, "RS-1", byID[a.Id].Code)
	require.Equal(t, "Combiphar", byID[b.Id].Name)
	require.Equal(t, "RS-2", byID[b.Id].Code)
}

func TestResolveSuppliers_EmptyInput(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.ResolveSuppliers(context.Background(), connect.NewRequest(&inventoryifacev1.ResolveSuppliersRequest{
		Ids: nil,
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Suppliers)
}
