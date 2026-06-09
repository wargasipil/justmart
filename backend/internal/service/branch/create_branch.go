package branch

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	branchifacev1 "github.com/justmart/backend/gen/branch_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *BranchService) CreateBranch(
	ctx context.Context,
	req *connect.Request[branchifacev1.CreateBranchRequest],
) (*connect.Response[branchifacev1.CreateBranchResponse], error) {
	code := strings.TrimSpace(strings.ToUpper(req.Msg.Code))
	name := strings.TrimSpace(req.Msg.Name)
	if code == "" || name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("code and name required"))
	}
	row := model.Branch{
		Code:    code,
		Name:    name,
		Address: strings.TrimSpace(req.Msg.Address),
		Phone:   strings.TrimSpace(req.Msg.Phone),
		Active:  true,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&branchifacev1.CreateBranchResponse{Branch: branchToProto(&row)}), nil
}
