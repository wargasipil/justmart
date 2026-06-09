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

func TestListUsers_ReturnsCreated(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	servicetest.EnsureOwner(t, gormDB, cfg) // one user (the owner) exists
	svc := usersvc.NewUserService(gormDB)

	// Add a second user so the list has >1 row.
	_, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "pharma@test.local",
		Name:     "Pharma",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_PHARMACIST,
	}))
	require.NoError(t, err)

	resp, err := svc.ListUsers(context.Background(), connect.NewRequest(&userifacev1.ListUsersRequest{}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.Msg.Users), 2)

	emails := map[string]bool{}
	for _, u := range resp.Msg.Users {
		emails[u.Email] = true
	}
	require.True(t, emails[servicetest.OwnerEmail])
	require.True(t, emails["pharma@test.local"])
}

func TestListUsers_EmptyDB(t *testing.T) {
	t.Parallel()
	// No users seeded (no EnsureOwner) -> empty, non-nil list, no error.
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.ListUsers(context.Background(), connect.NewRequest(&userifacev1.ListUsersRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Users)
	require.Empty(t, resp.Msg.Users)
}
