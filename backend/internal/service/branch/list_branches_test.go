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

func TestListBranches_SeededMain(t *testing.T) {
	t.Parallel()
	svc := branchsvc.NewBranchService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	// Migrations seed a single MAIN branch; no extra setup needed.
	resp, err := svc.ListBranches(context.Background(), connect.NewRequest(&branchifacev1.ListBranchesRequest{}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Branches)

	var found bool
	for _, b := range resp.Msg.Branches {
		require.True(t, b.Active) // include_inactive false -> only active rows
		if b.Code == "MAIN" {
			found = true
		}
	}
	require.True(t, found, "seeded MAIN branch should be listed")
}

func TestListBranches_ExcludesInactive(t *testing.T) {
	t.Parallel()
	svc := branchsvc.NewBranchService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	id := createBranch(t, svc, "INACT")
	_, err := svc.ArchiveBranch(context.Background(), connect.NewRequest(&branchifacev1.ArchiveBranchRequest{Id: id}))
	require.NoError(t, err)

	// Default (active-only): the archived branch must not appear.
	active, err := svc.ListBranches(context.Background(), connect.NewRequest(&branchifacev1.ListBranchesRequest{}))
	require.NoError(t, err)
	for _, b := range active.Msg.Branches {
		require.NotEqual(t, id, b.Id, "archived branch must be excluded by default")
	}

	// include_inactive: the archived branch must appear.
	all, err := svc.ListBranches(context.Background(), connect.NewRequest(&branchifacev1.ListBranchesRequest{
		IncludeInactive: true,
	}))
	require.NoError(t, err)
	var found bool
	for _, b := range all.Msg.Branches {
		if b.Id == id {
			found = true
		}
	}
	require.True(t, found, "archived branch should appear when include_inactive=true")
}
