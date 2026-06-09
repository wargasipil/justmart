package auth

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	coreauth "github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	usersvc "github.com/justmart/backend/internal/service/user"
)

func (s *AuthService) Login(
	ctx context.Context,
	req *connect.Request[userifacev1.LoginRequest],
) (*connect.Response[userifacev1.LoginResponse], error) {
	email := strings.TrimSpace(req.Msg.Email)
	if email == "" || req.Msg.Password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email and password required"))
	}

	if s.limiter != nil && !s.limiter.Allow(email) {
		return nil, connect.NewError(connect.CodeResourceExhausted,
			errors.New("too many login attempts; try again in a minute"))
	}

	var user model.User
	err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !user.Active {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("account disabled"))
	}
	if err := coreauth.VerifyPassword(user.PasswordHash, req.Msg.Password); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}

	accessTok, accessExp, err := s.access.Issue(user.ID, user.Role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	refreshTok, refreshExp, err := s.refresh.Mint(ctx, user.ID, nil, nil, req.Header().Get("User-Agent"))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&userifacev1.LoginResponse{
		AccessToken:      accessTok,
		RefreshToken:     refreshTok,
		User:             usersvc.UserToProto(&user),
		AccessExpiresAt:  accessExp.Unix(),
		RefreshExpiresAt: refreshExp.Unix(),
	}), nil
}
