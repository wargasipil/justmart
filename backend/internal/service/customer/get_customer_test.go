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

func TestGetCustomer_HappyPath(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	id := seedCustomer(t, svc, "Fajar", "0855000001")

	resp, err := svc.GetCustomer(context.Background(), connect.NewRequest(&customerifacev1.GetCustomerRequest{Id: id}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Customer)
	require.Equal(t, id, resp.Msg.Customer.Id)
	require.Equal(t, "Fajar", resp.Msg.Customer.Name)
	require.Equal(t, "0855000001", resp.Msg.Customer.Phone)
	require.True(t, resp.Msg.Customer.Active)
}

func TestGetCustomer_EmptyID(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.GetCustomer(context.Background(), connect.NewRequest(&customerifacev1.GetCustomerRequest{Id: ""}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestGetCustomer_NotFound(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.GetCustomer(context.Background(), connect.NewRequest(&customerifacev1.GetCustomerRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
