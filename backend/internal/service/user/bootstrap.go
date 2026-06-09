package user

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/model"
)

// EnsureBootstrapOwner upserts the owner described in config.Bootstrap.
// If owner_email is empty, it's a no-op (with a warning to the caller).
func (s *UserService) EnsureBootstrapOwner(ctx context.Context, b config.Bootstrap) error {
	if b.OwnerEmail == "" {
		fmt.Println("bootstrap: owner_email empty; skipping owner ensure")
		return nil
	}
	if b.OwnerPassword == "" {
		return fmt.Errorf("bootstrap: owner_password must be set when owner_email is set")
	}

	hash, err := auth.HashPassword(b.OwnerPassword)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}

	var existing model.User
	err = s.db.WithContext(ctx).Where("email = ?", b.OwnerEmail).First(&existing).Error
	var ownerID string
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		newUser := model.User{
			Email:        b.OwnerEmail,
			Name:         "Owner",
			PasswordHash: hash,
			Role:         roleOwner,
			Active:       true,
		}
		if err := s.db.WithContext(ctx).Create(&newUser).Error; err != nil {
			return fmt.Errorf("create bootstrap owner: %w", err)
		}
		ownerID = newUser.ID
		fmt.Printf("bootstrap: created owner %s\n", b.OwnerEmail)
	case err != nil:
		return fmt.Errorf("lookup bootstrap owner: %w", err)
	default:
		if err := s.db.WithContext(ctx).Model(&existing).Updates(map[string]any{
			"password_hash": hash,
			"role":          roleOwner,
			"active":        true,
		}).Error; err != nil {
			return fmt.Errorf("update bootstrap owner: %w", err)
		}
		ownerID = existing.ID
		fmt.Printf("bootstrap: ensured owner %s\n", b.OwnerEmail)
	}
	// Idempotent: grant the owner access to the default warehouse if no
	// existing membership for it. Heals owners created on a fresh DB where
	// migration 00019's at-migration-time grant had no users to grant.
	if err := grantDefaultWarehouse(s.db.WithContext(ctx), ownerID); err != nil {
		return fmt.Errorf("bootstrap owner default warehouse grant: %w", err)
	}
	return nil
}
