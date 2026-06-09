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

func TestCreateSupplier_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.CreateSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.CreateSupplierRequest{
		Code:         "sup-001", // lowercased on input -> uppercased by handler
		Name:         "Kimia Farma",
		ContactEmail: "sales@kimiafarma.co.id",
		Phone:        "0211234567",
	}))
	require.NoError(t, err)
	sup := resp.Msg.Supplier
	require.NotNil(t, sup)
	require.NotEmpty(t, sup.Id) // UUID filled by the SQLite create-callback
	require.Equal(t, "SUP-001", sup.Code)
	require.Equal(t, "Kimia Farma", sup.Name)
	require.Equal(t, "sales@kimiafarma.co.id", sup.ContactEmail)
	require.Equal(t, "0211234567", sup.Phone)
	require.True(t, sup.Active)
}

func TestCreateSupplier_MissingCodeOrName(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	// Blank (whitespace) name trims to empty -> InvalidArgument.
	_, err := svc.CreateSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.CreateSupplierRequest{
		Code: "SUP-002",
		Name: "   ",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateSupplier_DuplicateCode(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.CreateSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.CreateSupplierRequest{
		Code: "DUP-1",
		Name: "First",
	}))
	require.NoError(t, err)

	// Same code -> unique constraint -> handler maps to AlreadyExists.
	_, err = svc.CreateSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.CreateSupplierRequest{
		Code: "DUP-1",
		Name: "Second",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
}
