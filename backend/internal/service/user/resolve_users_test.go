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
	// Same ref shape as SearchUsers, so a resolved row can render the same
	// avatar + role badge a searched one does.
	require.Equal(t, authifacev1.Role_ROLE_CASHIER, resp.Msg.Users[0].Role)
	require.Zero(t, resp.Msg.Users[0].AvatarUpdatedAt)
}

// Resolve labels rows that already exist, so unlike SearchUsers it must still
// return a since-deactivated user — otherwise their past sales lose their name
// in the order-history "Created by" column.
func TestResolveUsers_IncludesDeactivated(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	ownerCtx := servicetest.OwnerCtx(context.Background(), ownerID)
	svc := usersvc.NewUserService(gormDB)

	created, err := svc.CreateUser(ownerCtx, connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "gone@test.local",
		Name:     "Gone Cashier",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)
	id := created.Msg.User.Id

	_, err = svc.SetUserActive(ownerCtx, connect.NewRequest(&userifacev1.SetUserActiveRequest{
		UserId: id,
		Active: false,
	}))
	require.NoError(t, err)

	resp, err := svc.ResolveUsers(ownerCtx, connect.NewRequest(&userifacev1.ResolveUsersRequest{
		Ids: []string{id},
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Users, 1)
	require.Equal(t, "Gone Cashier", resp.Msg.Users[0].Name)
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
