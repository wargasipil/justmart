package user_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	authifacev1 "github.com/justmart/backend/gen/auth_iface/v1"
	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	usersvc "github.com/justmart/backend/internal/service/user"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestCreateUser_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "cashier@test.local",
		Name:     "Cashier One",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)
	u := resp.Msg.User
	require.NotNil(t, u)
	require.NotEmpty(t, u.Id) // UUID filled by the SQLite create-callback
	require.Equal(t, "cashier@test.local", u.Email)
	require.Equal(t, "Cashier One", u.Name)
	require.Equal(t, authifacev1.Role_ROLE_CASHIER, u.Role)
	require.True(t, u.Active)
}

// The dedicated pharmacy role round-trips through roleFromProto/roleToProto.
func TestCreateUser_ApotekerRole(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "apoteker@test.local",
		Name:     "Apoteker One",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_APOTEKER,
	}))
	require.NoError(t, err)
	require.Equal(t, authifacev1.Role_ROLE_APOTEKER, resp.Msg.User.Role)
}

func TestCreateUser_ShortPassword(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "x@test.local",
		Password: "short", // < 8 chars -> InvalidArgument
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateUser_EmptyEmail(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "   ", // trimmed to empty -> InvalidArgument
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateUser_MissingRole(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "norole@test.local",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_UNSPECIFIED, // roleFromProto rejects -> InvalidArgument
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
