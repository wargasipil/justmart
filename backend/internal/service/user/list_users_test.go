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
	// Unpaged callers still get everything: NormPage's default page size applies.
	require.Equal(t, int32(len(resp.Msg.Users)), resp.Msg.Total)
}

// The users list is the one genuinely unbounded surface here, so paging must
// walk it exactly once and `total` must report the full count.
func TestListUsers_Paginates(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	servicetest.EnsureOwner(t, gormDB, cfg) // 1 user
	svc := usersvc.NewUserService(gormDB)
	ctx := context.Background()

	for _, e := range []string{"a@t.local", "b@t.local", "c@t.local", "d@t.local"} {
		_, err := svc.CreateUser(ctx, connect.NewRequest(&userifacev1.CreateUserRequest{
			Email: e, Name: e, Password: "supersecret", Role: authifacev1.Role_ROLE_CASHIER,
		}))
		require.NoError(t, err)
	}

	page := func(limit, offset int32) *userifacev1.ListUsersResponse {
		t.Helper()
		resp, err := svc.ListUsers(ctx, connect.NewRequest(&userifacev1.ListUsersRequest{
			Limit: limit, Offset: offset,
		}))
		require.NoError(t, err)
		return resp.Msg
	}

	first := page(2, 0)
	require.Equal(t, int32(5), first.Total) // owner + 4, not the page length
	require.Len(t, first.Users, 2)

	seen := map[string]bool{}
	for off := int32(0); off < first.Total; off += 2 {
		pg := page(2, off)
		require.Equal(t, first.Total, pg.Total) // total ignores the window
		for _, u := range pg.Users {
			require.False(t, seen[u.Id], "user %s appeared on two pages", u.Email)
			seen[u.Id] = true
		}
	}
	require.Len(t, seen, 5)

	// Past the end: empty page, total unchanged.
	require.Empty(t, page(2, 99).Users)
	require.Equal(t, int32(5), page(2, 99).Total)
}

// An APOTEKER user created in pharmacy mode stays visible after switching to
// retail — the mode-switch doesn't auto-downgrade or hide existing users; only
// NEW assignment of a pharmacy-only role is blocked. (Read path is mode-agnostic.)
func TestListUsers_ExistingApotekerVisibleInRetail(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := usersvc.NewUserService(db)

	require.NoError(t, common.SetBussinessType(context.Background(), db, common.BussinessTypePharmacyShop))
	_, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "apoteker@test.local",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_APOTEKER,
	}))
	require.NoError(t, err)

	// Shop switches to retail; the APOTEKER user must still list with its role.
	require.NoError(t, common.SetBussinessType(context.Background(), db, common.BussinessTypeRetail))
	resp, err := svc.ListUsers(context.Background(), connect.NewRequest(&userifacev1.ListUsersRequest{}))
	require.NoError(t, err)
	var found *authifacev1.Role
	for _, u := range resp.Msg.Users {
		if u.Email == "apoteker@test.local" {
			r := u.Role
			found = &r
		}
	}
	require.NotNil(t, found, "existing APOTEKER user should still be listed in retail")
	require.Equal(t, authifacev1.Role_ROLE_APOTEKER, *found)
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
