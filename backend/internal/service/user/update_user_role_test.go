package user_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	authifacev1 "github.com/justmart/backend/gen/auth_iface/v1"
	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/service/common"
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

// Promoting a user to the pharmacy-only APOTEKER role is rejected in retail mode
// (roles must not mix across modes).
func TestUpdateUserRole_ApotekerRejectedInRetailMode(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	require.NoError(t, common.SetBussinessType(context.Background(), db, common.BussinessTypeRetail))
	svc := usersvc.NewUserService(db)

	created, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "x@test.local",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)

	_, err = svc.UpdateUserRole(context.Background(), connect.NewRequest(&userifacev1.UpdateUserRoleRequest{
		UserId: created.Msg.User.Id,
		Role:   authifacev1.Role_ROLE_APOTEKER,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// In pharmacy mode the same promotion to APOTEKER succeeds.
func TestUpdateUserRole_ApotekerAllowedInPharmacyMode(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	require.NoError(t, common.SetBussinessType(context.Background(), db, common.BussinessTypePharmacyShop))
	svc := usersvc.NewUserService(db)

	created, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "x@test.local",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)

	resp, err := svc.UpdateUserRole(context.Background(), connect.NewRequest(&userifacev1.UpdateUserRoleRequest{
		UserId: created.Msg.User.Id,
		Role:   authifacev1.Role_ROLE_APOTEKER,
	}))
	require.NoError(t, err)
	require.Equal(t, authifacev1.Role_ROLE_APOTEKER, resp.Msg.User.Role)
}
