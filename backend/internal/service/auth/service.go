// Package auth implements user_iface.v1.AuthService (login / refresh / logout /
// me). The core JWT/refresh/limiter primitives live in internal/auth, imported
// here as coreauth to avoid the package-name clash. One RPC per file.
package auth

import (
	"gorm.io/gorm"

	coreauth "github.com/justmart/backend/internal/auth"
)

type AuthService struct {
	db      *gorm.DB
	access  *coreauth.Issuer
	refresh *coreauth.RefreshIssuer
	limiter *coreauth.LoginLimiter
}

func NewAuthService(
	db *gorm.DB,
	access *coreauth.Issuer,
	refresh *coreauth.RefreshIssuer,
	limiter *coreauth.LoginLimiter,
) *AuthService {
	return &AuthService{db: db, access: access, refresh: refresh, limiter: limiter}
}
