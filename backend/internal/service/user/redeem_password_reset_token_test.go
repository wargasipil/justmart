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

// issueResetToken seeds a target user + an OWNER-issued reset token, returning
// the raw token to redeem. Kept local to this test file.
func issueResetToken(t *testing.T, svc *usersvc.UserService, ownerCtx context.Context, email string) string {
	t.Helper()
	target, err := svc.CreateUser(context.Background(), connect.NewRequest(&userifacev1.CreateUserRequest{
		Email:    email,
		Password: "supersecret",
		Role:     authifacev1.Role_ROLE_CASHIER,
	}))
	require.NoError(t, err)
	issued, err := svc.IssuePasswordResetToken(ownerCtx, connect.NewRequest(&userifacev1.IssuePasswordResetTokenRequest{
		UserId: target.Msg.User.Id,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, issued.Msg.Token)
	return issued.Msg.Token
}

func TestRedeemPasswordResetToken_Success(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ownerCtx := servicetest.OwnerCtx(context.Background(), ownerID)

	raw := issueResetToken(t, svc, ownerCtx, "redeem@test.local")

	// Public RPC — no principal needed.
	_, err := svc.RedeemPasswordResetToken(context.Background(), connect.NewRequest(&userifacev1.RedeemPasswordResetTokenRequest{
		Token:       raw,
		NewPassword: "the-new-password",
	}))
	require.NoError(t, err)
}

func TestRedeemPasswordResetToken_AlreadyUsed(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := usersvc.NewUserService(gormDB)
	ownerCtx := servicetest.OwnerCtx(context.Background(), ownerID)

	raw := issueResetToken(t, svc, ownerCtx, "redeem-twice@test.local")

	// First redemption succeeds.
	_, err := svc.RedeemPasswordResetToken(context.Background(), connect.NewRequest(&userifacev1.RedeemPasswordResetTokenRequest{
		Token:       raw,
		NewPassword: "the-new-password",
	}))
	require.NoError(t, err)

	// Replay the same token -> Unauthenticated (already used).
	_, err = svc.RedeemPasswordResetToken(context.Background(), connect.NewRequest(&userifacev1.RedeemPasswordResetTokenRequest{
		Token:       raw,
		NewPassword: "another-new-password",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRedeemPasswordResetToken_InvalidToken(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.RedeemPasswordResetToken(context.Background(), connect.NewRequest(&userifacev1.RedeemPasswordResetTokenRequest{
		Token:       "deadbeefdeadbeefdeadbeefdeadbeef", // no matching hash
		NewPassword: "the-new-password",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRedeemPasswordResetToken_ShortPassword(t *testing.T) {
	t.Parallel()
	svc := usersvc.NewUserService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.RedeemPasswordResetToken(context.Background(), connect.NewRequest(&userifacev1.RedeemPasswordResetTokenRequest{
		Token:       "anything",
		NewPassword: "short", // < 8 chars -> InvalidArgument
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
