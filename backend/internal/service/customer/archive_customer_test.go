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

func TestArchiveCustomer_HappyPath(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	id := seedCustomer(t, svc, "Sari", "0811000001")

	resp, err := svc.ArchiveCustomer(context.Background(), connect.NewRequest(&customerifacev1.ArchiveCustomerRequest{Id: id}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Customer)
	require.Equal(t, id, resp.Msg.Customer.Id)
	require.False(t, resp.Msg.Customer.Active)

	// Confirm the archive persisted.
	got, err := svc.GetCustomer(context.Background(), connect.NewRequest(&customerifacev1.GetCustomerRequest{Id: id}))
	require.NoError(t, err)
	require.False(t, got.Msg.Customer.Active)
}

func TestArchiveCustomer_EmptyID(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.ArchiveCustomer(context.Background(), connect.NewRequest(&customerifacev1.ArchiveCustomerRequest{Id: ""}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestArchiveCustomer_NotFound(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.ArchiveCustomer(context.Background(), connect.NewRequest(&customerifacev1.ArchiveCustomerRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
