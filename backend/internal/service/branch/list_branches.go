package branch

import (
	"context"

	"connectrpc.com/connect"

	branchifacev1 "github.com/justmart/backend/gen/branch_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *BranchService) ListBranches(
	ctx context.Context,
	req *connect.Request[branchifacev1.ListBranchesRequest],
) (*connect.Response[branchifacev1.ListBranchesResponse], error) {
	q := s.db.WithContext(ctx).Order("code ASC")
	if !req.Msg.IncludeInactive {
		q = q.Where("active = ?", true)
	}
	var rows []model.Branch
	if err := q.Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*branchifacev1.Branch, 0, len(rows))
	for i := range rows {
		out = append(out, branchToProto(&rows[i]))
	}
	return connect.NewResponse(&branchifacev1.ListBranchesResponse{Branches: out}), nil
}
