package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	authifacev1 "github.com/justmart/backend/gen/auth_iface/v1"
	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// grantDefaultWarehouse ensures the user has a membership for the global
// default warehouse. Idempotent — safe to call on every boot. The membership's
// is_default is set true only when the user has no existing default (so a
// per-user default explicitly set later via SetDefaultWarehouse is preserved).
//
// Closes the gap in migration 00019_warehouses.sql, which grants every
// EXISTING user access to MAIN at migration time — users created after that
// (bootstrap owner, CreateUser admin path) used to land with zero memberships.
func grantDefaultWarehouse(tx *gorm.DB, userID string) error {
	var defaultWHID string
	err := tx.Table("warehouses").Select("id").Where("is_default").Limit(1).Scan(&defaultWHID).Error
	if err != nil {
		return fmt.Errorf("lookup default warehouse: %w", err)
	}
	if defaultWHID == "" {
		// No default warehouse seeded; nothing to grant.
		return nil
	}
	// If the user already has any default-flagged membership, don't disrupt it
	// — grant the default warehouse non-defaulted.
	var hasDefault bool
	err = tx.Raw(`SELECT EXISTS (
		SELECT 1 FROM user_warehouses WHERE user_id = ? AND is_default
	)`, userID).Scan(&hasDefault).Error
	if err != nil {
		return fmt.Errorf("check existing default: %w", err)
	}
	// INSERT ... ON CONFLICT DO NOTHING — re-running on an already-granted
	// user is a true no-op (keeps whatever is_default that row already has).
	res := tx.Exec(`
		INSERT INTO user_warehouses (user_id, warehouse_id, is_default)
		VALUES (?, ?, ?)
		ON CONFLICT (user_id, warehouse_id) DO NOTHING
	`, userID, defaultWHID, !hasDefault)
	if res.Error != nil {
		return fmt.Errorf("grant default warehouse: %w", res.Error)
	}
	return nil
}

func (s *UserService) loadByID(ctx context.Context, id string) (*model.User, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id required"))
	}
	var user model.User
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &user, nil
}

// UserToProto maps a user model to its proto. Exported because the auth service
// (Login / Me) returns the same shape.
func UserToProto(u *model.User) *userifacev1.User {
	return &userifacev1.User{
		Id:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Role:      roleToProto(u.Role),
		Active:    u.Active,
		CreatedAt: u.CreatedAt.Unix(),
	}
}

func roleFromProto(r authifacev1.Role) (string, error) {
	switch r {
	case authifacev1.Role_ROLE_OWNER:
		return roleOwner, nil
	case authifacev1.Role_ROLE_PHARMACIST:
		return rolePharmacist, nil
	case authifacev1.Role_ROLE_CASHIER:
		return roleCashier, nil
	case authifacev1.Role_ROLE_APOTEKER:
		return roleApoteker, nil
	default:
		return "", errors.New("role required")
	}
}

func roleToProto(s string) authifacev1.Role {
	switch s {
	case roleOwner:
		return authifacev1.Role_ROLE_OWNER
	case rolePharmacist:
		return authifacev1.Role_ROLE_PHARMACIST
	case roleCashier:
		return authifacev1.Role_ROLE_CASHIER
	case roleApoteker:
		return authifacev1.Role_ROLE_APOTEKER
	default:
		return authifacev1.Role_ROLE_UNSPECIFIED
	}
}

func generateResetToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	return raw, hashResetToken(raw), nil
}

func hashResetToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
