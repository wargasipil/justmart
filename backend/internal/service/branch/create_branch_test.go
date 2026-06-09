package branch_test

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	branchifacev1 "github.com/justmart/backend/gen/branch_iface/v1"
	branchsvc "github.com/justmart/backend/internal/service/branch"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestCreateBranch_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := branchsvc.NewBranchService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	// Unique code so a -count=N run never collides on the unique index.
	code := fmt.Sprintf("br-%d", 1)
	resp, err := svc.CreateBranch(context.Background(), connect.NewRequest(&branchifacev1.CreateBranchRequest{
		Code:    code,
		Name:    "Cabang Selatan",
		Address: "Jl. Selatan 10",
		Phone:   "0800111222",
	}))
	require.NoError(t, err)
	b := resp.Msg.Branch
	require.NotNil(t, b)
	require.NotEmpty(t, b.Id)                // UUID filled by the SQLite create-callback
	require.Equal(t, "BR-1", b.Code)         // handler upper-cases + trims the code
	require.Equal(t, "Cabang Selatan", b.Name)
	require.Equal(t, "Jl. Selatan 10", b.Address)
	require.Equal(t, "0800111222", b.Phone)
	require.True(t, b.Active)
}

func TestCreateBranch_MissingCodeOrName(t *testing.T) {
	t.Parallel()
	svc := branchsvc.NewBranchService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.CreateBranch(context.Background(), connect.NewRequest(&branchifacev1.CreateBranchRequest{
		Code: "  ", // trims to empty -> InvalidArgument
		Name: "Has Name",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
