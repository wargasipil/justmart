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

func TestSearchUsers_MatchByName(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "zara@test.local",
		Name:     "Zara Pharmacist",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_PHARMACIST,
	}))
	require.NoError(t, err)

	resp, err := svc.SearchUsers(context.Background(), connect.NewRequest(&userifacev1.SearchUsersRequest{
		Query: "Zara",
		Limit: 10,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Users, 1)
	require.Equal(t, "zara@test.local", resp.Msg.Users[0].Email)
	require.Equal(t, "Zara Pharmacist", resp.Msg.Users[0].Name)
	require.NotEmpty(t, resp.Msg.Users[0].Id)
}

func TestSearchUsers_MatchByEmail(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "needle@test.local",
		Name:     "Some Person",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)

	resp, err := svc.SearchUsers(context.Background(), connect.NewRequest(&userifacev1.SearchUsersRequest{
		Query: "needle",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Users, 1)
	require.Equal(t, "needle@test.local", resp.Msg.Users[0].Email)
}

func TestSearchUsers_NoMatch(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)

	resp, err := svc.SearchUsers(context.Background(), connect.NewRequest(&userifacev1.SearchUsersRequest{
		Query: "no-such-user-zzzz",
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Users)
	require.Empty(t, resp.Msg.Users)
}
