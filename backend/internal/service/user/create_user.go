package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
)

func (s *UserService) CreateUser(
	ctx context.Context,
	req *connect.Request[userifacev1.CreateUserRequest],
) (*connect.Response[userifacev1.CreateUserResponse], error) {
	m := req.Msg
	email := strings.TrimSpace(m.Email)
	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email required"))
	}
	if len(m.Password) < 8 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("password must be at least 8 characters"))
	}
	roleStr, err := roleFromProto(m.Role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	hash, err := auth.HashPassword(m.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	user := model.User{
		Email:        email,
		Name:         strings.TrimSpace(m.Name),
		PasswordHash: hash,
		Role:         roleStr,
		Active:       true,
	}
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("create user: %w", err))
	}
	// Grant the new user access to the default warehouse (their first usable
	// location). Owners can later grant access to additional warehouses via
	// the /warehouses admin UI.
	if err := grantDefaultWarehouse(s.db.WithContext(ctx), user.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("grant default warehouse: %w", err))
	}

	return connect.NewResponse(&userifacev1.CreateUserResponse{User: UserToProto(&user)}), nil
}
