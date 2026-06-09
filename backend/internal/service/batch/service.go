// Package batch implements inventory_iface.v1.BatchService. One RPC per file;
// this file holds the struct + constructor. Stock-locking and per-warehouse
// quantity helpers live in service/common.
package batch

import "gorm.io/gorm"

type BatchService struct {
	db *gorm.DB
}

func NewBatchService(db *gorm.DB) *BatchService { return &BatchService{db: db} }
