package branch

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	branchifacev1 "github.com/justmart/backend/gen/branch_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *BranchService) GrantBranchAccess(
	ctx context.Context,
	req *connect.Request[branchifacev1.GrantBranchAccessRequest],
) (*connect.Response[branchifacev1.GrantBranchAccessResponse], error) {
	if req.Msg.UserId == "" || req.Msg.BranchId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id and branch_id required"))
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Upsert membership.
		mem := model.UserBranch{
			UserID:    req.Msg.UserId,
			BranchID:  req.Msg.BranchId,
			IsDefault: req.Msg.IsDefault,
		}
		if err := tx.Save(&mem).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if req.Msg.IsDefault {
			// Clear other defaults for this user.
			if err := tx.Model(&model.UserBranch{}).
				Where("user_id = ? AND branch_id <> ?", req.Msg.UserId, req.Msg.BranchId).
				Update("is_default", false).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	return connect.NewResponse(&branchifacev1.GrantBranchAccessResponse{
		Membership: &branchifacev1.UserBranchMembership{
			UserId:    req.Msg.UserId,
			BranchId:  req.Msg.BranchId,
			IsDefault: req.Msg.IsDefault,
		},
	}), nil
}
