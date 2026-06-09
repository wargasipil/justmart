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

func TestListUserBranches_Self(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := branchsvc.NewBranchService(gormDB)

	// Grant the owner a MAIN-branch membership so the list is non-empty.
	_, err := svc.GrantBranchAccess(context.Background(), connect.NewRequest(&branchifacev1.GrantBranchAccessRequest{
		UserId:    ownerID,
		BranchId:  mainBranchID,
		IsDefault: true,
	}))
	require.NoError(t, err)

	// Empty user_id in the request -> resolves to the caller (self).
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	resp, err := svc.ListUserBranches(ctx, connect.NewRequest(&branchifacev1.ListUserBranchesRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Memberships, 1)
	require.Equal(t, ownerID, resp.Msg.Memberships[0].UserId)
	require.Equal(t, mainBranchID, resp.Msg.Memberships[0].BranchId)
	require.True(t, resp.Msg.Memberships[0].IsDefault)
	// Branches are hydrated alongside the memberships.
	require.Len(t, resp.Msg.Branches, 1)
	require.Equal(t, mainBranchID, resp.Msg.Branches[0].Id)
}

func TestListUserBranches_Unauthenticated(t *testing.T) {
	t.Parallel()
	svc := branchsvc.NewBranchService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	// No principal in ctx -> MustPrincipal returns Unauthenticated.
	_, err := svc.ListUserBranches(context.Background(), connect.NewRequest(&branchifacev1.ListUserBranchesRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestListUserBranches_OtherUserDeniedForNonOwner(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := branchsvc.NewBranchService(gormDB)

	// A CASHIER asking about a different user -> PermissionDenied.
	ctx := servicetest.CtxAs(context.Background(), "CASHIER", "00000000-0000-0000-0000-00000000c0de")
	_, err := svc.ListUserBranches(ctx, connect.NewRequest(&branchifacev1.ListUserBranchesRequest{
		UserId: ownerID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}
