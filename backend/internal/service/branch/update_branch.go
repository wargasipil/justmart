package branch

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	branchifacev1 "github.com/justmart/backend/gen/branch_iface/v1"
)

func (s *BranchService) UpdateBranch(
	ctx context.Context,
	req *connect.Request[branchifacev1.UpdateBranchRequest],
) (*connect.Response[branchifacev1.UpdateBranchResponse], error) {
	row, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name required"))
	}
	updates := map[string]any{
		"name":    name,
		"address": strings.TrimSpace(req.Msg.Address),
		"phone":   strings.TrimSpace(req.Msg.Phone),
	}
	if err := s.db.WithContext(ctx).Model(row).Updates(updates).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	row, err = s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&branchifacev1.UpdateBranchResponse{Branch: branchToProto(row)}), nil
}
