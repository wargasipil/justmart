// Package table implements table_iface.v1.TableService — the restaurant floor:
// the dining-table catalog (manager tier) plus the floor actions a waiter
// performs during service (open a table, move an order).
//
// The design decision worth knowing before reading anything here: an OPEN BILL
// IS A DRAFT SALE. There is no order entity, no extra sale status and no new
// state machine — a table is occupied exactly when a DRAFT sale carries its id,
// and a partial unique index (see migration 00055) makes "one open bill per
// table" a database guarantee rather than a read-then-write check. Settling the
// bill is the ordinary CompleteSale, which is why WAITER is not granted it.
package table

import "gorm.io/gorm"

type TableService struct {
	db *gorm.DB
}

func NewTableService(db *gorm.DB) *TableService { return &TableService{db: db} }
