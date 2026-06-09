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

func TestRevokeBranchAccess_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := branchsvc.NewBranchService(gormDB)

	// Seed a membership to revoke.
	_, err := svc.GrantBranchAccess(context.Background(), connect.NewRequest(&branchifacev1.GrantBranchAccessRequest{
		UserId:   ownerID,
		BranchId: mainBranchID,
	}))
	require.NoError(t, err)

	_, err = svc.RevokeBranchAccess(context.Background(), connect.NewRequest(&branchifacev1.RevokeBranchAccessRequest{
		UserId:   ownerID,
		BranchId: mainBranchID,
	}))
	require.NoError(t, err)

	// Revoking again -> NotFound (row already gone).
	_, err = svc.RevokeBranchAccess(context.Background(), connect.NewRequest(&branchifacev1.RevokeBranchAccessRequest{
		UserId:   ownerID,
		BranchId: mainBranchID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestRevokeBranchAccess_NotFound(t *testing.T) {
	t.Parallel()
	svc := branchsvc.NewBranchService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.RevokeBranchAccess(context.Background(), connect.NewRequest(&branchifacev1.RevokeBranchAccessRequest{
		UserId:   "00000000-0000-0000-0000-000000000abc",
		BranchId: mainBranchID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
