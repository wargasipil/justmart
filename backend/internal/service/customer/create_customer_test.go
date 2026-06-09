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

func TestCreateCustomer_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.CreateCustomer(context.Background(), connect.NewRequest(&customerifacev1.CreateCustomerRequest{
		Name:    "Budi Santoso",
		Phone:   "0811222333",
		Address: "Jl. Merdeka 1",
		Notes:   "regular",
	}))
	require.NoError(t, err)
	c := resp.Msg.Customer
	require.NotNil(t, c)
	require.NotEmpty(t, c.Id) // UUID filled by the SQLite create-callback
	require.Equal(t, "Budi Santoso", c.Name)
	require.Equal(t, "0811222333", c.Phone)
	require.Equal(t, "Jl. Merdeka 1", c.Address)
	require.Equal(t, "regular", c.Notes)
	require.True(t, c.Active)
}

func TestCreateCustomer_EmptyName(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.CreateCustomer(context.Background(), connect.NewRequest(&customerifacev1.CreateCustomerRequest{
		Name: "   ", // trimmed to empty -> CodeInvalidArgument
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
