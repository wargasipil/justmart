package model

import "time"

type PurchaseOrder struct {
	ID           string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	PoNo         *string    `gorm:"uniqueIndex;column:po_no"`
	SupplierID   string     `gorm:"not null;type:uuid;column:supplier_id"`
	Status       string     `gorm:"not null;default:'DRAFT'"`
	InvoiceNo    string     `gorm:"not null;default:'';column:invoice_no"`
	InvoiceDate  *time.Time `gorm:"type:date;column:invoice_date"`
	DueAt        *time.Time `gorm:"type:date;column:due_at"`
	Note         string     `gorm:"not null;default:''"`
	Subtotal     int64      `gorm:"not null;default:0;column:subtotal"`
	CartDiscount int64      `gorm:"not null;default:0;column:cart_discount"`
	PpnEnabled   bool       `gorm:"not null;default:false;column:ppn_enabled"`
	PpnRate      int32      `gorm:"not null;default:11;column:ppn_rate"` // percent (0-100); ignored when PpnEnabled=false
	PpnAmount    int64      `gorm:"not null;default:0;column:ppn_amount"`
	OrderedTotal int64      `gorm:"not null;default:0;column:ordered_total"` // = Subtotal − CartDiscount + PpnAmount
	PaidAmount   int64      `gorm:"not null;default:0;column:paid_amount"`
	// Accumulated value of purchase returns. Outstanding = OrderedTotal −
	// PaidAmount − ReturnedAmount (may go negative = a supplier credit).
	ReturnedAmount int64    `gorm:"not null;default:0;column:returned_amount"`
	CreatedBy    string     `gorm:"not null;type:uuid;column:created_by"`
	BranchID     *string    `gorm:"type:uuid;column:branch_id"`
	WarehouseID  string     `gorm:"not null;type:uuid;column:warehouse_id"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	SentAt       *time.Time `gorm:"column:sent_at"`
	ClosedAt     *time.Time `gorm:"column:closed_at"`

	Items []PurchaseOrderItem `gorm:"foreignKey:PurchaseOrderID"`
}

func (PurchaseOrder) TableName() string { return "purchase_orders" }

type PurchaseOrderItem struct {
	ID              string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	PurchaseOrderID string `gorm:"not null;type:uuid;column:purchase_order_id"`
	ProductID      string `gorm:"not null;type:uuid;column:product_id"`
	OrderedQty      int32  `gorm:"not null;column:ordered_qty"`  // BASE units
	ReceivedQty     int32  `gorm:"not null;default:0;column:received_qty"` // BASE units
	UnitCostPrice   int64  `gorm:"not null;default:0;column:unit_cost_price"` // per BASE unit
	Subtotal        int64  `gorm:"not null;default:0"`
	// Purchasable unit the line was ordered in (display/entry metadata).
	ProductUnitID *string `gorm:"type:uuid;column:product_unit_id"`
	UnitName       string  `gorm:"not null;default:'';column:unit_name"`
	UnitFactor     int64   `gorm:"not null;default:1;column:unit_factor"`
	// Per-line discount. DiscountValue is minor units when FIXED, basis points
	// (percent*100) when PERCENT. Subtotal above is the NET (gross − discount).
	DiscountType  string `gorm:"not null;default:'FIXED';column:discount_type"`
	DiscountValue int64  `gorm:"not null;default:0;column:discount_value"`
	// DiscountPerItem: when true the discount applies to each item's cost (× qty)
	// instead of the whole line. Combines with DiscountType for 4 effective modes.
	DiscountPerItem bool `gorm:"not null;default:false;column:discount_per_item"`
}

func (PurchaseOrderItem) TableName() string { return "purchase_order_items" }

type PurchaseReceipt struct {
	ID              string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ReceiptNo       *string   `gorm:"uniqueIndex;column:receipt_no"`
	PurchaseOrderID string    `gorm:"not null;type:uuid;column:purchase_order_id"`
	ReceivedAt      time.Time `gorm:"not null;type:date;column:received_at"`
	ReceivedBy      string    `gorm:"not null;type:uuid;column:received_by"`
	Note            string    `gorm:"not null;default:''"`
	InvoiceNo       string    `gorm:"not null;default:'';column:invoice_no"`
	CreatedAt       time.Time
	// Cancelled ("batal terima") — the receipt was entered in error. The row and
	// its RCV number survive as a voided document, but its lots and their stock
	// movements are deleted and received_qty is reversed. Only ever set while
	// every lot was still untouched; see CancelReceipt.
	VoidedAt   *time.Time `gorm:"column:voided_at"`
	VoidedBy   *string    `gorm:"type:uuid;column:voided_by"`
	VoidReason string     `gorm:"not null;default:'';column:void_reason"`

	Items []PurchaseReceiptItem `gorm:"foreignKey:PurchaseReceiptID"`
}

func (PurchaseReceipt) TableName() string { return "purchase_receipts" }

type PurchaseReceiptItem struct {
	ID                  string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	PurchaseReceiptID   string    `gorm:"not null;type:uuid;column:purchase_receipt_id"`
	PurchaseOrderItemID string    `gorm:"not null;type:uuid;column:purchase_order_item_id"`
	ProductID          string    `gorm:"not null;type:uuid;column:product_id"`
	Qty                 int32     `gorm:"not null"` // BASE units
	UnitCostPrice       int64     `gorm:"not null;default:0;column:unit_cost_price"` // per BASE unit
	BatchNumber         string    `gorm:"not null;default:'';column:batch_number"`
	ExpiryDate          time.Time `gorm:"not null;type:date;column:expiry_date"`
	BatchID             *string   `gorm:"type:uuid;column:batch_id"`
	CreatedAt           time.Time
	// Purchasable unit the line was received in (display/entry metadata).
	ProductUnitID *string `gorm:"type:uuid;column:product_unit_id"`
	UnitName       string  `gorm:"not null;default:'';column:unit_name"`
	UnitFactor     int64   `gorm:"not null;default:1;column:unit_factor"`
}

func (PurchaseReceiptItem) TableName() string { return "purchase_receipt_items" }

type POCounter struct {
	Year    int `gorm:"primaryKey"`
	LastSeq int `gorm:"not null;default:0;column:last_seq"`
}

func (POCounter) TableName() string { return "po_no_counters" }

type RcvCounter struct {
	Year    int `gorm:"primaryKey"`
	LastSeq int `gorm:"not null;default:0;column:last_seq"`
}

func (RcvCounter) TableName() string { return "rcv_no_counters" }

// PurchaseReturn is a partial "retur pembelian": received goods sent back to the
// supplier. Append-only ledger mirroring PurchaseReceipt. Each item reverses a
// receipt line (one batch) via a negative PURCHASE_RETURN stock movement.
type PurchaseReturn struct {
	ID              string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ReturnNo        *string   `gorm:"uniqueIndex;column:return_no"` // RTN-YYYY-NNNN
	PurchaseOrderID string    `gorm:"not null;type:uuid;column:purchase_order_id"`
	WarehouseID     string    `gorm:"not null;type:uuid;column:warehouse_id"`
	ReturnedAt      time.Time `gorm:"not null;type:date;column:returned_at"`
	ReturnedBy      string    `gorm:"not null;type:uuid;column:returned_by"`
	Reason          string    `gorm:"not null;default:''"`
	Note            string    `gorm:"not null;default:''"`
	RefundAmount    int64     `gorm:"not null;default:0;column:refund_amount"` // Σ line (qty × unit_cost_price)
	CreatedAt       time.Time

	Items []PurchaseReturnItem `gorm:"foreignKey:PurchaseReturnID"`
}

func (PurchaseReturn) TableName() string { return "purchase_returns" }

type PurchaseReturnItem struct {
	ID                    string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	PurchaseReturnID      string `gorm:"not null;type:uuid;column:purchase_return_id"`
	PurchaseReceiptItemID string `gorm:"not null;type:uuid;column:purchase_receipt_item_id"`
	PurchaseOrderItemID   string `gorm:"not null;type:uuid;column:purchase_order_item_id"`
	ProductID             string `gorm:"not null;type:uuid;column:product_id"`
	BatchID               string `gorm:"not null;type:uuid;column:batch_id"`
	Qty                   int32  `gorm:"not null"` // BASE units returned
	UnitCostPrice         int64  `gorm:"not null;default:0;column:unit_cost_price"` // per BASE unit
	UnitName              string `gorm:"not null;default:'';column:unit_name"`
	UnitFactor            int64  `gorm:"not null;default:1;column:unit_factor"`
}

func (PurchaseReturnItem) TableName() string { return "purchase_return_items" }

type RtnCounter struct {
	Year    int `gorm:"primaryKey"`
	LastSeq int `gorm:"not null;default:0;column:last_seq"`
}

func (RtnCounter) TableName() string { return "rtn_no_counters" }
