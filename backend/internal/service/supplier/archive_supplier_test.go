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

func TestArchiveSupplier_DeactivatesRow(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	seeded := seedSupplier(t, svc, "AR-1", "ToArchive")
	require.True(t, seeded.Active)

	resp, err := svc.ArchiveSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.ArchiveSupplierRequest{
		Id: seeded.Id,
	}))
	require.NoError(t, err)
	require.False(t, resp.Msg.Supplier.Active)

	// Persisted: a re-Get reports inactive.
	got, err := svc.GetSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.GetSupplierRequest{Id: seeded.Id}))
	require.NoError(t, err)
	require.False(t, got.Msg.Supplier.Active)
}

func TestArchiveSupplier_NotFound(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.ArchiveSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.ArchiveSupplierRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
