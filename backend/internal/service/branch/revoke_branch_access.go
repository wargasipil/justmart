package branch

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	branchifacev1 "github.com/justmart/backend/gen/branch_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *BranchService) RevokeBranchAccess(
	ctx context.Context,
	req *connect.Request[branchifacev1.RevokeBranchAccessRequest],
) (*connect.Response[branchifacev1.RevokeBranchAccessResponse], error) {
	res := s.db.WithContext(ctx).Where("user_id = ? AND branch_id = ?",
		req.Msg.UserId, req.Msg.BranchId).Delete(&model.UserBranch{})
	if res.Error != nil {
		return nil, connect.NewError(connect.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("membership not found"))
	}
	return connect.NewResponse(&branchifacev1.RevokeBranchAccessResponse{}), nil
}
