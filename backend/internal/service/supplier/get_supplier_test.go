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

// seedSupplier creates one supplier via the real CreateSupplier RPC and returns
// its proto. Shared by the supplier_test package's tests.
func seedSupplier(t *testing.T, svc *suppliersvc.SupplierService, code, name string) *inventoryifacev1.Supplier {
	t.Helper()
	resp, err := svc.CreateSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.CreateSupplierRequest{
		Code: code,
		Name: name,
	}))
	require.NoError(t, err)
	return resp.Msg.Supplier
}

func TestGetSupplier_Found(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	seeded := seedSupplier(t, svc, "GET-1", "Phapros")

	resp, err := svc.GetSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.GetSupplierRequest{
		Id: seeded.Id,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Supplier)
	require.Equal(t, seeded.Id, resp.Msg.Supplier.Id)
	require.Equal(t, "GET-1", resp.Msg.Supplier.Code)
	require.Equal(t, "Phapros", resp.Msg.Supplier.Name)
}

func TestGetSupplier_NotFound(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.GetSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.GetSupplierRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSupplier_EmptyID(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.GetSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.GetSupplierRequest{Id: ""}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
