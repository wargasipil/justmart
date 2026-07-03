package model

import "time"

// ProductLastRestock holds the LATEST restock of a product from a supplier in a
// warehouse — one row per (warehouse_id, product_id, supplier_id), upserted by
// CreateReceipt. Drives the supplier-detail product list + the product-list
// "last restock" columns. Discount is stored as type+value (FIXED minor units |
// PERCENT basis points), mirroring the purchasing/POS discount convention.
type ProductLastRestock struct {
	ID                string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	WarehouseID       string    `gorm:"not null;type:uuid;column:warehouse_id"`
	ProductID         string    `gorm:"not null;type:uuid;column:product_id"`
	SupplierID        string    `gorm:"not null;type:uuid;column:supplier_id"`
	LastPrice         int64     `gorm:"not null;default:0;column:last_price"` // NET unit cost per base unit
	LastQty           int64     `gorm:"not null;default:0;column:last_qty"`   // base units received
	LastDiscountType    string  `gorm:"not null;default:'FIXED';column:last_discount_type"`
	LastDiscountValue   int64   `gorm:"not null;default:0;column:last_discount_value"`
	LastDiscountPerItem bool    `gorm:"not null;default:false;column:last_discount_per_item"`
	LastCreatedAt     time.Time `gorm:"not null;column:last_created_at"` // PO (restock order) created
	LastArrivedAt     time.Time `gorm:"not null;column:last_arrived_at"` // receipt received_at
	UpdatedAt         time.Time
}

func (ProductLastRestock) TableName() string { return "product_last_restocks" }

// ProductRestockLog is the append-only restock history — one row per receipt
// line, inserted by CreateReceipt. Drives the product-detail "restock price
// history" tab. created/arrived mirror the PO-created / receipt-received times.
type ProductRestockLog struct {
	ID               string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	WarehouseID      string    `gorm:"not null;type:uuid;column:warehouse_id"`
	ProductID        string    `gorm:"not null;type:uuid;column:product_id"`
	SupplierID       string    `gorm:"not null;type:uuid;column:supplier_id"`
	Price            int64     `gorm:"not null;default:0;column:price"`
	Qty              int64     `gorm:"not null;default:0;column:qty"`
	DiscountType     string    `gorm:"not null;default:'FIXED';column:discount_type"`
	DiscountValue    int64     `gorm:"not null;default:0;column:discount_value"`
	DiscountPerItem  bool      `gorm:"not null;default:false;column:discount_per_item"`
	RestockCreatedAt time.Time `gorm:"not null;column:restock_created_at"` // PO created
	RestockArrivedAt time.Time `gorm:"not null;column:restock_arrived_at"` // receipt received_at
	ReceiptID        *string   `gorm:"type:uuid;column:receipt_id"`
	CreatedAt        time.Time
}

func (ProductRestockLog) TableName() string { return "product_restock_logs" }
