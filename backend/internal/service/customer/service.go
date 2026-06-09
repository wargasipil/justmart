// Package customer implements customer_iface.v1.CustomerService. Each RPC lives
// in its own file (e.g. list_customers.go); this file holds the service struct
// and its constructor. Cross-cutting helpers come from service/common.
package customer

import "gorm.io/gorm"

type CustomerService struct {
	db *gorm.DB
}

func NewCustomerService(db *gorm.DB) *CustomerService { return &CustomerService{db: db} }
