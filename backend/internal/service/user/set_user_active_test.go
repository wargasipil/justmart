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

func TestSetUserActive_Toggle(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	created, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "toggle@test.local",
		Name:     "Toggle",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)
	require.True(t, created.Msg.User.Active)
	id := created.Msg.User.Id

	// Deactivate.
	off, err := svc.SetUserActive(context.Background(), connect.NewRequest(&userifacev1.SetUserActiveRequest{
		UserId: id,
		Active: false,
	}))
	require.NoError(t, err)
	require.False(t, off.Msg.User.Active)
	require.Equal(t, id, off.Msg.User.Id)

	// Reactivate.
	on, err := svc.SetUserActive(context.Background(), connect.NewRequest(&userifacev1.SetUserActiveRequest{
		UserId: id,
		Active: true,
	}))
	require.NoError(t, err)
	require.True(t, on.Msg.User.Active)
}

func TestSetUserActive_NotFound(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.SetUserActive(context.Background(), connect.NewRequest(&userifacev1.SetUserActiveRequest{
		UserId: "00000000-0000-0000-0000-000000000000",
		Active: false,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestSetUserActive_EmptyID(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.SetUserActive(context.Background(), connect.NewRequest(&userifacev1.SetUserActiveRequest{
		UserId: "", // loadByID rejects -> InvalidArgument
		Active: false,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
