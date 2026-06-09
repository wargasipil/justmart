package user

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
)

func (s *UserService) IssuePasswordResetToken(
	ctx context.Context,
	req *connect.Request[userifacev1.IssuePasswordResetTokenRequest],
) (*connect.Response[userifacev1.IssuePasswordResetTokenResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.UserId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id required"))
	}
	var target model.User
	if err := s.db.WithContext(ctx).Where("id = ?", req.Msg.UserId).First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	raw, hash, err := generateResetToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	exp := time.Now().Add(passwordResetTTL)
	row := model.PasswordResetToken{
		UserID:    target.ID,
		TokenHash: hash,
		IssuedBy:  caller.UserID,
		ExpiresAt: exp,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&userifacev1.IssuePasswordResetTokenResponse{
		Token:     raw,
		ExpiresAt: exp.Unix(),
	}), nil
}
