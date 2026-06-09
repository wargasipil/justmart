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

// createBranch is a small local helper: creates a branch and returns its id.
func createBranch(t *testing.T, svc *branchsvc.BranchService, code string) string {
	t.Helper()
	resp, err := svc.CreateBranch(context.Background(), connect.NewRequest(&branchifacev1.CreateBranchRequest{
		Code: code,
		Name: "Seed " + code,
	}))
	require.NoError(t, err)
	return resp.Msg.Branch.Id
}

func TestUpdateBranch_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := branchsvc.NewBranchService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	id := createBranch(t, svc, "UPD")

	resp, err := svc.UpdateBranch(context.Background(), connect.NewRequest(&branchifacev1.UpdateBranchRequest{
		Id:      id,
		Name:    "Renamed Branch",
		Address: "Jl. Baru 5",
		Phone:   "0899000111",
	}))
	require.NoError(t, err)
	b := resp.Msg.Branch
	require.NotNil(t, b)
	require.Equal(t, id, b.Id)
	require.Equal(t, "Renamed Branch", b.Name)
	require.Equal(t, "Jl. Baru 5", b.Address)
	require.Equal(t, "0899000111", b.Phone)
	require.Equal(t, "UPD", b.Code) // code is not changed by UpdateBranch
}

func TestUpdateBranch_NotFound(t *testing.T) {
	t.Parallel()
	svc := branchsvc.NewBranchService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.UpdateBranch(context.Background(), connect.NewRequest(&branchifacev1.UpdateBranchRequest{
		Id:   "00000000-0000-0000-0000-000000000999",
		Name: "Nope",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestUpdateBranch_EmptyName(t *testing.T) {
	t.Parallel()
	svc := branchsvc.NewBranchService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	id := createBranch(t, svc, "UPD2")

	_, err := svc.UpdateBranch(context.Background(), connect.NewRequest(&branchifacev1.UpdateBranchRequest{
		Id:   id,
		Name: "   ", // trims to empty -> InvalidArgument
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
