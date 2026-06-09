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

func TestResolveUsers_ByIDs(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	created, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "resolveme@test.local",
		Name:     "Resolve Me",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)
	id := created.Msg.User.Id

	resp, err := svc.ResolveUsers(context.Background(), connect.NewRequest(&userifacev1.ResolveUsersRequest{
		Ids: []string{id, id}, // duplicate -> deduped to one row
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Users, 1)
	require.Equal(t, id, resp.Msg.Users[0].Id)
	require.Equal(t, "resolveme@test.local", resp.Msg.Users[0].Email)
	require.Equal(t, "Resolve Me", resp.Msg.Users[0].Name)
}

func TestResolveUsers_EmptyIDs(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.ResolveUsers(context.Background(), connect.NewRequest(&userifacev1.ResolveUsersRequest{
		Ids: nil, // empty -> early return, empty list
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Users)
}

func TestResolveUsers_UnknownIDIgnored(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.ResolveUsers(context.Background(), connect.NewRequest(&userifacev1.ResolveUsersRequest{
		Ids: []string{"00000000-0000-0000-0000-000000000000"}, // no such user
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Users)
}
