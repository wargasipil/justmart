package user_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	authifacev1 "github.com/justmart/backend/gen/auth_iface/v1"
	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/service/servicetest"
	usersvc "github.com/justmart/backend/internal/service/user"
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

// The ref carries role + avatar marker so a picker can render a role badge and
// an avatar without a second round-trip. A user who never uploaded a picture
// reports 0, which is what disables the avatar fetch client-side.
func TestSearchUsers_ReturnsRoleAndAvatarMarker(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "badge@test.local",
		Name:     "Badge Cashier",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)

	resp, err := svc.SearchUsers(context.Background(), connect.NewRequest(&userifacev1.SearchUsersRequest{
		Query: "badge",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Users, 1)
	require.Equal(t, authifacev1.Role_ROLE_CASHIER, resp.Msg.Users[0].Role)
	require.Zero(t, resp.Msg.Users[0].AvatarUpdatedAt, "no picture uploaded => 0")
}

// A deactivated account is not a valid pick for any caller — it must never
// surface in a picker, even on an exact-name query.
func TestSearchUsers_ExcludesDeactivated(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	ownerCtx := servicetest.OwnerCtx(context.Background(), ownerID)
	svc := usersvc.NewUserService(gormDB)

	created, err := svc.CreateUser(ownerCtx, connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "exstaff@test.local",
		Name:     "Ex Staff",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)

	// Visible while active.
	resp, err := svc.SearchUsers(ownerCtx, connect.NewRequest(&userifacev1.SearchUsersRequest{
		Query: "Ex Staff",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Users, 1)

	_, err = svc.SetUserActive(ownerCtx, connect.NewRequest(&userifacev1.SetUserActiveRequest{
		UserId: created.Msg.User.Id,
		Active: false,
	}))
	require.NoError(t, err)

	resp, err = svc.SearchUsers(ownerCtx, connect.NewRequest(&userifacev1.SearchUsersRequest{
		Query: "Ex Staff",
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Users, "deactivated user must not be pickable")
}

// with_sales_only backs the cashier-scope filter: every option it offers must
// yield rows, so a user with no COMPLETED sale is excluded — while the same
// query without the flag still returns them for the grant/authoring pickers.
func TestSearchUsers_WithSalesOnly(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)

	seller, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "seller@test.local",
		Name:     "Sells Things",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)
	_, err = svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    "idle@test.local",
		Name:     "Never Sold",
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)

	var wh model.Warehouse
	require.NoError(t, gormDB.Where("is_default").First(&wh).Error)
	whID := wh.ID
	now := time.Now()

	// One COMPLETED sale for the seller, one DRAFT for the owner — a draft cart
	// is not evidence of selling and must not qualify.
	require.NoError(t, gormDB.Create(&model.Sale{
		CashierUserID: seller.Msg.User.Id,
		WarehouseID:   &whID,
		Total:         10_000,
		Status:        common.SaleStatusCompleted,
		CompletedAt:   &now,
	}).Error)
	require.NoError(t, gormDB.Create(&model.Sale{
		CashierUserID: ownerID,
		WarehouseID:   &whID,
		Status:        "DRAFT",
	}).Error)

	resp, err := svc.SearchUsers(context.Background(), connect.NewRequest(&userifacev1.SearchUsersRequest{
		WithSalesOnly: true,
	}))
	require.NoError(t, err)
	emails := make([]string, 0, len(resp.Msg.Users))
	for _, u := range resp.Msg.Users {
		emails = append(emails, u.Email)
	}
	require.Equal(t, []string{"seller@test.local"}, emails)

	// Unset, the same roster query returns everyone active.
	resp, err = svc.SearchUsers(context.Background(), connect.NewRequest(&userifacev1.SearchUsersRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Users, 3, "owner + seller + idle")
}
