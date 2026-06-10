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

func TestUpdateSupplier_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	seeded := seedSupplier(t, svc, "UP-1", "Old Name")

	resp, err := svc.UpdateSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.UpdateSupplierRequest{
		Id:                seeded.Id,
		Code:              "up-2", // lowercased -> uppercased by handler
		Name:              "New Name",
		ContactEmail:      "ops@example.com",
		Phone:             "0800000000",
		Address:           "Jl. Baru No. 1",
		BankName:          "Mandiri",
		BankAccountNumber: "9876543210",
		BankAccountHolder: "CV Sumber Sehat",
	}))
	require.NoError(t, err)
	sup := resp.Msg.Supplier
	require.Equal(t, seeded.Id, sup.Id)
	require.Equal(t, "UP-2", sup.Code)
	require.Equal(t, "New Name", sup.Name)
	require.Equal(t, "ops@example.com", sup.ContactEmail)
	require.Equal(t, "0800000000", sup.Phone)
	require.Equal(t, "Jl. Baru No. 1", sup.Address)
	require.Equal(t, "Mandiri", sup.BankName)
	require.Equal(t, "9876543210", sup.BankAccountNumber)
	require.Equal(t, "CV Sumber Sehat", sup.BankAccountHolder)

	// Persisted: a re-Get reflects the update (incl. the new fields).
	got, err := svc.GetSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.GetSupplierRequest{Id: seeded.Id}))
	require.NoError(t, err)
	require.Equal(t, "New Name", got.Msg.Supplier.Name)
	require.Equal(t, "Jl. Baru No. 1", got.Msg.Supplier.Address)
	require.Equal(t, "9876543210", got.Msg.Supplier.BankAccountNumber)
}

func TestUpdateSupplier_NotFound(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UpdateSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.UpdateSupplierRequest{
		Id:   "00000000-0000-0000-0000-000000000000",
		Code: "X-1",
		Name: "Ghost",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestUpdateSupplier_MissingCodeOrName(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	seeded := seedSupplier(t, svc, "UP-3", "Keep")

	// Blank name (whitespace) trims to empty -> InvalidArgument.
	_, err := svc.UpdateSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.UpdateSupplierRequest{
		Id:   seeded.Id,
		Code: "UP-3",
		Name: "   ",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
