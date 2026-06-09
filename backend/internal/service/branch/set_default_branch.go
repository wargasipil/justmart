package branch

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	branchifacev1 "github.com/justmart/backend/gen/branch_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *BranchService) SetDefaultBranch(
	ctx context.Context,
	req *connect.Request[branchifacev1.SetDefaultBranchRequest],
) (*connect.Response[branchifacev1.SetDefaultBranchResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	target := req.Msg.UserId
	if target == "" {
		target = caller.UserID
	}
	if target != caller.UserID && caller.Role != "OWNER" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("can only set own default"))
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var mem model.UserBranch
		if err := tx.Where("user_id = ? AND branch_id = ?", target, req.Msg.BranchId).
			First(&mem).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodePermissionDenied,
					errors.New("user has no access to that branch"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := tx.Model(&model.UserBranch{}).
			Where("user_id = ?", target).Update("is_default", false).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := tx.Model(&model.UserBranch{}).
			Where("user_id = ? AND branch_id = ?", target, req.Msg.BranchId).
			Update("is_default", true).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	return connect.NewResponse(&branchifacev1.SetDefaultBranchResponse{
		Membership: &branchifacev1.UserBranchMembership{
			UserId:    target,
			BranchId:  req.Msg.BranchId,
			IsDefault: true,
		},
	}), nil
}
