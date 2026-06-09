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

// seedCustomer is a small local helper that creates one customer via the real
// handler and returns its id. Kept local to the package's tests.
func seedCustomer(t *testing.T, svc *customersvc.CustomerService, name, phone string) string {
	t.Helper()
	resp, err := svc.CreateCustomer(context.Background(), connect.NewRequest(&customerifacev1.CreateCustomerRequest{
		Name:  name,
		Phone: phone,
	}))
	require.NoError(t, err)
	return resp.Msg.Customer.Id
}

func TestListCustomers_HappyPath(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	seedCustomer(t, svc, "Andi", "0811000001")
	seedCustomer(t, svc, "Budi", "0811000002")

	resp, err := svc.ListCustomers(context.Background(), connect.NewRequest(&customerifacev1.ListCustomersRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(2), resp.Msg.Total)
	require.Len(t, resp.Msg.Customers, 2)
	// Ordered by name ascending.
	require.Equal(t, "Andi", resp.Msg.Customers[0].Name)
	require.Equal(t, "Budi", resp.Msg.Customers[1].Name)
}

func TestListCustomers_QueryFilter(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	seedCustomer(t, svc, "Charlie", "0822000001")
	seedCustomer(t, svc, "Dewi", "0833000002")

	// Filter by name substring — only Charlie should match.
	resp, err := svc.ListCustomers(context.Background(), connect.NewRequest(&customerifacev1.ListCustomersRequest{
		Query: "harl",
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Len(t, resp.Msg.Customers, 1)
	require.Equal(t, "Charlie", resp.Msg.Customers[0].Name)
}

func TestListCustomers_ExcludesInactiveByDefault(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	id := seedCustomer(t, svc, "Eka", "0844000001")
	_, err := svc.ArchiveCustomer(context.Background(), connect.NewRequest(&customerifacev1.ArchiveCustomerRequest{Id: id}))
	require.NoError(t, err)

	// Default: inactive excluded.
	resp, err := svc.ListCustomers(context.Background(), connect.NewRequest(&customerifacev1.ListCustomersRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(0), resp.Msg.Total)

	// IncludeInactive surfaces the archived row.
	resp, err = svc.ListCustomers(context.Background(), connect.NewRequest(&customerifacev1.ListCustomersRequest{
		IncludeInactive: true,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.False(t, resp.Msg.Customers[0].Active)
}
