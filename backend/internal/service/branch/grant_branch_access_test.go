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

// mainBranchID is the fixed id of the MAIN branch seeded by the SQLite init
// migration (see migrations/sqlite/00001_init.sql).
const mainBranchID = "00000000-0000-0000-0000-0000000000b1"

func TestGrantBranchAccess_RoundTrip(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg) // a real users.id for the FK
	svc := branchsvc.NewBranchService(gormDB)

	resp, err := svc.GrantBranchAccess(context.Background(), connect.NewRequest(&branchifacev1.GrantBranchAccessRequest{
		UserId:    ownerID,
		BranchId:  mainBranchID,
		IsDefault: true,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Membership)
	require.Equal(t, ownerID, resp.Msg.Membership.UserId)
	require.Equal(t, mainBranchID, resp.Msg.Membership.BranchId)
	require.True(t, resp.Msg.Membership.IsDefault)
}

func TestGrantBranchAccess_MissingArgs(t *testing.T) {
	t.Parallel()
	svc := branchsvc.NewBranchService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	_, err := svc.GrantBranchAccess(context.Background(), connect.NewRequest(&branchifacev1.GrantBranchAccessRequest{
		UserId: "", // missing -> InvalidArgument
		BranchId: mainBranchID,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
