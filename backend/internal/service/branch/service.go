// Package branch implements branch_iface.v1.BranchService. (Branches are
// dormant/deprecated — superseded by warehouses — but the service is kept
// registered.) One RPC per file; this file holds the struct + constructor.
package branch

import "gorm.io/gorm"

type BranchService struct {
	db *gorm.DB
}

func NewBranchService(db *gorm.DB) *BranchService { return &BranchService{db: db} }
