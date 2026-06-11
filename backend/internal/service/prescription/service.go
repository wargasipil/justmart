// Package prescription implements prescription_iface.v1.PrescriptionService.
// One RPC per file; this file holds the struct + constructor. Non-RPC helpers
// (status computation, rx_no assignment, proto mapping, row loads) live in
// helpers.go.
//
// Pharmacy-mode domain (resep): a prescription anchors to a customer, lists
// prescribed products with per-line dispensed accumulation, and is the gate POS
// uses to dispense Rx-required products. Status is computed read-through
// (only ACTIVE/VOIDED are stored). See helpers.computeRxStatus.
package prescription

import "gorm.io/gorm"

type PrescriptionService struct {
	db *gorm.DB
}

func NewPrescriptionService(db *gorm.DB) *PrescriptionService {
	return &PrescriptionService{db: db}
}
