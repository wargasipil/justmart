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

func TestUpdateUserRole_Promote(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	created, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "promote@test.local",
		Name:     "Promote",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)
	id := created.Msg.User.Id

	resp, err := svc.UpdateUserRole(context.Background(), connect.NewRequest(&userifacev1.UpdateUserRoleRequest{
		UserId: id,
		Role:   authifacev1.Role_ROLE_PHARMACIST,
	}))
	require.NoError(t, err)
	require.Equal(t, id, resp.Msg.User.Id)
	require.Equal(t, authifacev1.Role_ROLE_PHARMACIST, resp.Msg.User.Role)
}

func TestUpdateUserRole_InvalidRole(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	created, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "badrole@test.local",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)

	_, err = svc.UpdateUserRole(context.Background(), connect.NewRequest(&userifacev1.UpdateUserRoleRequest{
		UserId: created.Msg.User.Id,
		Role:   authifacev1.Role_ROLE_UNSPECIFIED, // roleFromProto rejects -> InvalidArgument
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateUserRole_NotFound(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UpdateUserRole(context.Background(), connect.NewRequest(&userifacev1.UpdateUserRoleRequest{
		UserId: "00000000-0000-0000-0000-000000000000",
		Role:   authifacev1.Role_ROLE_PHARMACIST,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
