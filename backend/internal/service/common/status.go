package common

// Sale status / payment-source / movement-type string constants (the values
// stored in the DB). Shared by SaleService, AnalyticsService, and the draft
// sweeper.
const (
	SaleStatusDraft     = "DRAFT"
	SaleStatusCompleted = "COMPLETED"
	SaleStatusVoided    = "VOIDED"

	PaymentCash    = "CASH"
	PaymentNonCash = "NON_CASH"

	MovementTypeSale = "SALE"
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
