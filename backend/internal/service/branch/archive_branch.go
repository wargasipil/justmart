package branch

import (
	"context"

	"connectrpc.com/connect"

	branchifacev1 "github.com/justmart/backend/gen/branch_iface/v1"
)

func (s *BranchService) ArchiveBranch(
	ctx context.Context,
	req *connect.Request[branchifacev1.ArchiveBranchRequest],
) (*connect.Response[branchifacev1.ArchiveBranchResponse], error) {
	row, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(row).Update("active", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row.Active = false
	return connect.NewResponse(&branchifacev1.ArchiveBranchResponse{Branch: branchToProto(row)}), nil
}
