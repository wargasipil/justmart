package branch

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	branchifacev1 "github.com/justmart/backend/gen/branch_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
)

func (s *BranchService) ListUserBranches(
	ctx context.Context,
	req *connect.Request[branchifacev1.ListUserBranchesRequest],
) (*connect.Response[branchifacev1.ListUserBranchesResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	target := req.Msg.UserId
	if target == "" {
		target = caller.UserID
	}
	// Non-owners can only ask about themselves.
	if target != caller.UserID && caller.Role != "OWNER" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("can only list own memberships"))
	}

	var mems []model.UserBranch
	if err := s.db.WithContext(ctx).Where("user_id = ?", target).Find(&mems).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(mems) == 0 {
		return connect.NewResponse(&branchifacev1.ListUserBranchesResponse{}), nil
	}
	ids := make([]string, 0, len(mems))
	for _, m := range mems {
		ids = append(ids, m.BranchID)
	}
	var branches []model.Branch
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&branches).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	outMems := make([]*branchifacev1.UserBranchMembership, 0, len(mems))
	for _, m := range mems {
		outMems = append(outMems, &branchifacev1.UserBranchMembership{
			UserId:    m.UserID,
			BranchId:  m.BranchID,
			IsDefault: m.IsDefault,
		})
	}
	outBranches := make([]*branchifacev1.Branch, 0, len(branches))
	for i := range branches {
		outBranches = append(outBranches, branchToProto(&branches[i]))
	}
	return connect.NewResponse(&branchifacev1.ListUserBranchesResponse{
		Memberships: outMems,
		Branches:    outBranches,
	}), nil
}
