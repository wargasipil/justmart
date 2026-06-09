package user

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *UserService) RedeemPasswordResetToken(
	ctx context.Context,
	req *connect.Request[userifacev1.RedeemPasswordResetTokenRequest],
) (*connect.Response[userifacev1.RedeemPasswordResetTokenResponse], error) {
	if strings.TrimSpace(req.Msg.Token) == "" || len(req.Msg.NewPassword) < 8 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("token and new_password (min 8 chars) required"))
	}
	hash := hashResetToken(req.Msg.Token)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.PasswordResetToken
		if err := tx.Where("token_hash = ?", hash).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if row.UsedAt != nil {
			return connect.NewError(connect.CodeUnauthenticated, errors.New("token already used"))
		}
		if time.Now().After(row.ExpiresAt) {
			return connect.NewError(connect.CodeUnauthenticated, errors.New("token expired"))
		}
		newHash, err := auth.HashPassword(req.Msg.NewPassword)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		now := time.Now()
		if err := tx.Model(&model.User{}).Where("id = ?", row.UserID).
			Update("password_hash", newHash).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := tx.Model(&row).Update("used_at", now).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	return connect.NewResponse(&userifacev1.RedeemPasswordResetTokenResponse{}), nil
}
