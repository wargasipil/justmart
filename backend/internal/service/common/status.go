package common

// Sale status / payment-source / movement-type string constants (the values
// stored in the DB). Shared by SaleService, AnalyticsService, and the draft
// sweeper.
const (
	SaleStatusDraft     = "DRAFT"
	SaleStatusCompleted = "COMPLETED"
	SaleStatusVoided    = "VOIDED"
	SaleStatusRefunded  = "REFUNDED"

	PaymentCash    = "CASH"
	PaymentNonCash = "NON_CASH"

	MovementTypeSale = "SALE"
	// MovementTypeReturn is a positive movement returning goods to stock on a
	// full-order refund (mirrors the sale's SALE movements). Not counted as COGS
	// (analytics keys COGS off type='SALE').
	MovementTypeReturn = "RETURN"
)

// Purchase-order status string constants (DB values). Shared by the purchasing
// services and by ProductService (open-PO "on order" stock).
const (
	POStatusDraft             = "DRAFT"
	POStatusSent              = "SENT"
	POStatusPartiallyReceived = "PARTIALLY_RECEIVED"
	POStatusReceived          = "RECEIVED"
	POStatusClosed            = "CLOSED"
	POStatusVoided            = "VOIDED"
)
