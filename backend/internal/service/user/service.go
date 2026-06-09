// Package user implements user_iface.v1.UserService. One RPC per file; this
// file holds the struct, constructor, and user-domain constants. EnsureBootstrap
// Owner (a boot-time helper, not an RPC) lives in bootstrap.go.
package user

import (
	"time"

	"gorm.io/gorm"
)

const passwordResetTTL = 24 * time.Hour

const (
	roleOwner      = "OWNER"
	rolePharmacist = "PHARMACIST"
	roleCashier    = "CASHIER"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService { return &UserService{db: db} }
