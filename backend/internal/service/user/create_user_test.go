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

func TestCreateUser_DuplicateEmail(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email: "dup@test.local", Name: "First", Password: "supersecret", Role: authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)

	_, err = svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email: "dup@test.local", Name: "Second", Password: "supersecret", Role: authifacev1.Role_ROLE_CASHIER,
	}))
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, connect.CodeAlreadyExists, ce.Code())
	require.Equal(t, "user.email_taken", ce.Message())
}

// APOTEKER (the pharmacy-only Rx-authority role) is creatable in pharmacy mode
// and round-trips through roleFromProto/roleToProto.
func TestCreateUser_ApotekerAllowedInPharmacyMode(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	require.NoError(t, common.SetBussinessType(context.Background(), db, common.BussinessTypePharmacyShop))
	svc := usersvc.NewUserService(db)

	resp, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "apoteker@test.local",
		Name:     "Apoteker One",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_APOTEKER,
	}))
	require.NoError(t, err)
	require.Equal(t, authifacev1.Role_ROLE_APOTEKER, resp.Msg.User.Role)
}

// In retail mode the pharmacy-only APOTEKER role is rejected — roles must not
// mix across modes (test-specification.md › User).
func TestCreateUser_ApotekerRejectedInRetailMode(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	require.NoError(t, common.SetBussinessType(context.Background(), db, common.BussinessTypeRetail))
	svc := usersvc.NewUserService(db)

	_, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "apoteker@test.local",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_APOTEKER,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// WAITER (the restaurant-only floor role) is creatable in restaurant mode and
// round-trips through roleFromProto/roleToProto.
func TestCreateUser_WaiterAllowedInRestaurantMode(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	require.NoError(t, common.SetBussinessType(context.Background(), db, common.BussinessTypeRestaurant))
	svc := usersvc.NewUserService(db)

	resp, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "waiter@test.local",
		Name:     "Pramusaji One",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_WAITER,
	}))
	require.NoError(t, err)
	require.Equal(t, authifacev1.Role_ROLE_WAITER, resp.Msg.User.Role)
}

// Roles must not mix across modes in EITHER direction: the restaurant-only
// WAITER is rejected in retail/pharmacy, and the pharmacy-only APOTEKER is
// rejected in restaurant.
func TestCreateUser_ModeSpecificRolesDoNotCross(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		mode int32
		role authifacev1.Role
	}{
		{"waiter in retail", common.BussinessTypeRetail, authifacev1.Role_ROLE_WAITER},
		{"waiter in pharmacy", common.BussinessTypePharmacyShop, authifacev1.Role_ROLE_WAITER},
		{"apoteker in restaurant", common.BussinessTypeRestaurant, authifacev1.Role_ROLE_APOTEKER},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := servicetest.NewDB(t, servicetest.NewConfig(t))
			require.NoError(t, common.SetBussinessType(context.Background(), db, tc.mode))
			svc := usersvc.NewUserService(db)

			_, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
				Email:    "crossmode@test.local",
				Password: "supersecret",
				Role:     tc.role,
			}))
			require.Error(t, err)
			require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		})
	}
}

// The shared roles (OWNER, PHARMACIST/"Admin", CASHIER) are all creatable in
// retail — no false rejections. (Default UNSPECIFIED mode also behaves as retail.)
func TestCreateUser_SharedRolesAllowedInRetail(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	require.NoError(t, common.SetBussinessType(context.Background(), db, common.BussinessTypeRetail))
	svc := usersvc.NewUserService(db)

	shared := []struct {
		email string
		role  authifacev1.Role
	}{
		{"owner2@test.local", authifacev1.Role_ROLE_OWNER},
		{"admin@test.local", authifacev1.Role_ROLE_PHARMACIST},
		{"cashier2@test.local", authifacev1.Role_ROLE_CASHIER},
	}
	for _, c := range shared {
		_, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
			Email:    c.email,
			Password: "supersecret",
			Role:     c.role,
		}))
		require.NoErrorf(t, err, "role %v should be allowed in retail", c.role)
	}
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
