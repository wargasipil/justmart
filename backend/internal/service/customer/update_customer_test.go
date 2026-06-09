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

func TestUpdateCustomer_HappyPath(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	id := seedCustomer(t, svc, "Putra", "0811000001")

	resp, err := svc.UpdateCustomer(context.Background(), connect.NewRequest(&customerifacev1.UpdateCustomerRequest{
		Id:      id,
		Name:    "Putra Wijaya",
		Phone:   "0822999888",
		Address: "Jl. Sudirman 10",
		Notes:   "VIP",
	}))
	require.NoError(t, err)
	c := resp.Msg.Customer
	require.NotNil(t, c)
	require.Equal(t, id, c.Id)
	require.Equal(t, "Putra Wijaya", c.Name)
	require.Equal(t, "0822999888", c.Phone)
	require.Equal(t, "Jl. Sudirman 10", c.Address)
	require.Equal(t, "VIP", c.Notes)
}

func TestUpdateCustomer_EmptyName(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	id := seedCustomer(t, svc, "Rara", "0811000001")

	_, err := svc.UpdateCustomer(context.Background(), connect.NewRequest(&customerifacev1.UpdateCustomerRequest{
		Id:   id,
		Name: "   ", // trimmed to empty -> CodeInvalidArgument
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateCustomer_NotFound(t *testing.T) {
	t.Parallel()
	svc := customersvc.NewCustomerService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UpdateCustomer(context.Background(), connect.NewRequest(&customerifacev1.UpdateCustomerRequest{
		Id:   "00000000-0000-0000-0000-000000000000",
		Name: "Ghost",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
