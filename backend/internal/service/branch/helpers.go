package branch

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	branchifacev1 "github.com/justmart/backend/gen/branch_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func (s *BranchService) load(ctx context.Context, id string) (*model.Branch, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var row model.Branch
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("branch not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &row, nil
}

func branchToProto(b *model.Branch) *branchifacev1.Branch {
	return &branchifacev1.Branch{
		Id:        b.ID,
		Code:      b.Code,
		Name:      b.Name,
		Address:   b.Address,
		Phone:     b.Phone,
		Active:    b.Active,
		CreatedAt: b.CreatedAt.Unix(),
	}
}
