package branch_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	branchifacev1 "github.com/justmart/backend/gen/branch_iface/v1"
	branchsvc "github.com/justmart/backend/internal/service/branch"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestSetDefaultBranch_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := branchsvc.NewBranchService(gormDB)

	// Grant access (non-default) first, then promote to default.
	_, err := svc.GrantBranchAccess(context.Background(), connect.NewRequest(&branchifacev1.GrantBranchAccessRequest{
		UserId:   ownerID,
		BranchId: mainBranchID,
	}))
	require.NoError(t, err)

	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	resp, err := svc.SetDefaultBranch(ctx, connect.NewRequest(&branchifacev1.SetDefaultBranchRequest{
		BranchId: mainBranchID, // empty user_id -> self
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Membership)
	require.Equal(t, ownerID, resp.Msg.Membership.UserId)
	require.Equal(t, mainBranchID, resp.Msg.Membership.BranchId)
	require.True(t, resp.Msg.Membership.IsDefault)
}

func TestSetDefaultBranch_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc := branchsvc.NewBranchService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.SetDefaultBranch(context.Background(), connect.NewRequest(&branchifacev1.SetDefaultBranchRequest{
		BranchId: mainBranchID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestSetDefaultBranch_NoAccess(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := branchsvc.NewBranchService(gormDB)

	// Caller has no user_branches row for MAIN -> PermissionDenied.
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	_, err := svc.SetDefaultBranch(ctx, connect.NewRequest(&branchifacev1.SetDefaultBranchRequest{
		BranchId: mainBranchID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}
