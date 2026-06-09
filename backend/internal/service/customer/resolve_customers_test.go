package customer_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	customerifacev1 "github.com/justmart/backend/gen/customer_iface/v1"
	customersvc "github.com/justmart/backend/internal/service/customer"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestResolveCustomers_HappyPath(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	id1 := seedCustomer(t, svc, "Maya", "0811000001")
	id2 := seedCustomer(t, svc, "Nanda", "0811000002")

	resp, err := svc.ResolveCustomers(context.Background(), connect.NewRequest(&customerifacev1.ResolveCustomersRequest{
		Ids: []string{id1, id2},
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Customers, 2)

	byID := map[string]string{}
	for _, ref := range resp.Msg.Customers {
		byID[ref.Id] = ref.Name
	}
	require.Equal(t, "Maya", byID[id1])
	require.Equal(t, "Nanda", byID[id2])
}

func TestResolveCustomers_EmptyInput(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.ResolveCustomers(context.Background(), connect.NewRequest(&customerifacev1.ResolveCustomersRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Customers)
}

func TestResolveCustomers_UnknownIDsOmitted(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	id := seedCustomer(t, svc, "Oka", "0811000001")

	resp, err := svc.ResolveCustomers(context.Background(), connect.NewRequest(&customerifacev1.ResolveCustomersRequest{
		Ids: []string{id, "00000000-0000-0000-0000-000000000000"},
	}))
	require.NoError(t, err)
	// Unknown id is silently dropped; only the real one comes back.
	require.Len(t, resp.Msg.Customers, 1)
	require.Equal(t, id, resp.Msg.Customers[0].Id)
	require.Equal(t, "Oka", resp.Msg.Customers[0].Name)
}
