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

func TestSearchSuppliers_MatchesByNameAndCode(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	seedSupplier(t, svc, "SRCH-1", "Indofarma")
	seedSupplier(t, svc, "SRCH-2", "Phapros")

	// Match by name fragment.
	resp, err := svc.SearchSuppliers(context.Background(), connect.NewRequest(&inventoryifacev1.SearchSuppliersRequest{
		Query: "Indo",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Suppliers, 1)
	require.Equal(t, "Indofarma", resp.Msg.Suppliers[0].Name)

	// Match by code fragment.
	respCode, err := svc.SearchSuppliers(context.Background(), connect.NewRequest(&inventoryifacev1.SearchSuppliersRequest{
		Query: "SRCH-2",
	}))
	require.NoError(t, err)
	require.Len(t, respCode.Msg.Suppliers, 1)
	require.Equal(t, "Phapros", respCode.Msg.Suppliers[0].Name)
}

func TestSearchSuppliers_EmptyQueryReturnsActive(t *testing.T) {
	t.Parallel()
	svc := suppliersvc.NewSupplierService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	seedSupplier(t, svc, "SE-1", "Soho Global")
	archived := seedSupplier(t, svc, "SE-2", "Otsuka")
	_, err := svc.ArchiveSupplier(context.Background(), connect.NewRequest(&inventoryifacev1.ArchiveSupplierRequest{
		Id: archived.Id,
	}))
	require.NoError(t, err)

	// Empty query lists active suppliers only.
	resp, err := svc.SearchSuppliers(context.Background(), connect.NewRequest(&inventoryifacev1.SearchSuppliersRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Suppliers, 1)
	require.Equal(t, "Soho Global", resp.Msg.Suppliers[0].Name)
}
