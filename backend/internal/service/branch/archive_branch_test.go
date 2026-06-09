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

func TestArchiveBranch_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := branchsvc.NewBranchService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	id := createBranch(t, svc, "ARC")

	resp, err := svc.ArchiveBranch(context.Background(), connect.NewRequest(&branchifacev1.ArchiveBranchRequest{
		Id: id,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Branch)
	require.Equal(t, id, resp.Msg.Branch.Id)
	require.False(t, resp.Msg.Branch.Active) // archived -> active flipped off
}

func TestArchiveBranch_NotFound(t *testing.T) {
	t.Parallel()
	svc := branchsvc.NewBranchService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.ArchiveBranch(context.Background(), connect.NewRequest(&branchifacev1.ArchiveBranchRequest{
		Id: "00000000-0000-0000-0000-000000000999",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
