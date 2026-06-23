package user

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *UserService) CreateUser(
	ctx context.Context,
	req *connect.Request[userifacev1.CreateUserRequest],
) (*connect.Response[userifacev1.CreateUserResponse], error) {
	m := req.Msg
	email := strings.TrimSpace(m.Email)
	if email == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "user.email_required")
	}
	if len(m.Password) < 8 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "user.password_too_short")
	}
	roleStr, err := roleFromProto(m.Role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := s.validateRoleForMode(ctx, roleStr); err != nil {
		return nil, err
	}

	hash, err := auth.HashPassword(m.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Pre-check uniqueness for a specific token (the DB unique index — citext on
	// Postgres — backstops any case-variant race the plain compare misses).
	if taken, e := common.ExistsBy(s.db.WithContext(ctx), &model.User{}, "email = ?", email); e != nil {
		return nil, connect.NewError(connect.CodeInternal, e)
	} else if taken {
		return nil, common.TokenError(connect.CodeAlreadyExists, "user.email_taken")
	}

	user := model.User{
		Email:        email,
		Name:         strings.TrimSpace(m.Name),
		PasswordHash: hash,
		Role:         roleStr,
		Active:       true,
	}
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, common.TokenError(connect.CodeAlreadyExists, "user.email_taken")
	}
	// Grant the new user access to the default warehouse (their first usable
	// location). Owners can later grant access to additional warehouses via
	// the /warehouses admin UI.
	if err := grantDefaultWarehouse(s.db.WithContext(ctx), user.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("grant default warehouse: %w", err))
	}

	return connect.NewResponse(&userifacev1.CreateUserResponse{User: UserToProto(&user)}), nil
}
