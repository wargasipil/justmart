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

func TestUnarchiveSupplier_SetsActive(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	seeded := seedSupplier(t, svc, "UN-1", "ToRestore")

	_, err := svc.ArchiveSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.ArchiveSupplierRequest{Id: seeded.Id}))
	require.NoError(t, err)

	resp, err := svc.UnarchiveSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.UnarchiveSupplierRequest{Id: seeded.Id}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Supplier.Active)

	// Persisted: a re-Get reports active.
	got, err := svc.GetSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.GetSupplierRequest{Id: seeded.Id}))
	require.NoError(t, err)
	require.True(t, got.Msg.Supplier.Active)
}

// Restoring is rejected when another ACTIVE supplier has taken the name in the
// meantime (the partial unique index on (name) WHERE active = true).
func TestUnarchiveSupplier_NameCollisionRejected(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	first := seedSupplier(t, svc, "ACME-1", "Acme")
	_, err := svc.ArchiveSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.ArchiveSupplierRequest{Id: first.Id}))
	require.NoError(t, err)

	// Now a NEW active supplier can take the same name (the first is archived).
	_ = seedSupplier(t, svc, "ACME-2", "Acme")

	// Unarchiving the first would create two active "Acme" rows → reject.
	_, err = svc.UnarchiveSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.UnarchiveSupplierRequest{Id: first.Id}))
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, connect.CodeAlreadyExists, ce.Code())
	require.Equal(t, "supplier.name_taken", ce.Message())
}

func TestUnarchiveSupplier_NotFound(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UnarchiveSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.UnarchiveSupplierRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
