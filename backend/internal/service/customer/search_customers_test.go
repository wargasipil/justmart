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

func TestSearchCustomers_ByName(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	seedCustomer(t, svc, "Gilang", "0866000001")
	seedCustomer(t, svc, "Hadi", "0877000002")

	resp, err := svc.SearchCustomers(context.Background(), connect.NewRequest(&customerifacev1.SearchCustomersRequest{
		Query: "ilan",
		Limit: 10,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Customers, 1)
	require.Equal(t, "Gilang", resp.Msg.Customers[0].Name)
}

func TestSearchCustomers_ByPhone(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	seedCustomer(t, svc, "Indra", "0899111222")
	seedCustomer(t, svc, "Joko", "0888333444")

	resp, err := svc.SearchCustomers(context.Background(), connect.NewRequest(&customerifacev1.SearchCustomersRequest{
		Query: "111222",
		Limit: 10,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Customers, 1)
	require.Equal(t, "Indra", resp.Msg.Customers[0].Name)
}

func TestSearchCustomers_EmptyQueryReturnsActiveCustomers(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	seedCustomer(t, svc, "Kiki", "0811000001")
	id := seedCustomer(t, svc, "Lina", "0811000002")
	_, err := svc.ArchiveCustomer(context.Background(), connect.NewRequest(&customerifacev1.ArchiveCustomerRequest{Id: id}))
	require.NoError(t, err)

	// No query -> all ACTIVE customers (archived Lina excluded).
	resp, err := svc.SearchCustomers(context.Background(), connect.NewRequest(&customerifacev1.SearchCustomersRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Customers, 1)
	require.Equal(t, "Kiki", resp.Msg.Customers[0].Name)
}
