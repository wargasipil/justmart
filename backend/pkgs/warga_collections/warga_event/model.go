package warga_event

import "time"

// Topic is a named channel messages are published to. Created on demand by the
// first Send or Pull that references it.
type Topic struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time `gorm:"not null"`
}

// Subscription is a durable consumer bound to exactly one topic. Created on
// demand by the first Pull that references it.
type Subscription struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"uniqueIndex;not null"`
	Topic     string    `gorm:"index;not null"`
	CreatedAt time.Time `gorm:"not null"`
}

// Delivery is one message destined for one subscription. Publishing a message
// to a topic with N subscriptions creates N Delivery rows that share a
// MessageID (the fan-out copies) but each carry their own opaque AckID.
//
// Lifecycle: pending (LeasedAt nil, Acked false) -> leased (Pull sent it and set
// LeasedAt) -> acked (consumer called Ack). A leased-but-unacked row whose lease
// is older than the ack deadline becomes eligible for redelivery; an acked row
// is reclaimed by Maintain after the retention window.
type Delivery struct {
	ID           uint   `gorm:"primaryKey"`
	AckID        string `gorm:"uniqueIndex;not null"`
	MessageID    string `gorm:"index;not null"`
	Topic        string `gorm:"index;not null"`
	Subscription string `gorm:"index:idx_delivery_pull,priority:1;not null"`
	Data         []byte
	// Attributes is the JSON-encoded map<string,string> of message attributes.
	Attributes  string
	PublishTime time.Time `gorm:"not null"`
	Acked       bool      `gorm:"index:idx_delivery_pull,priority:2;not null;default:false"`
	// LeasedAt is when the message was last streamed to a consumer (nil = never
	// sent). Used as the redelivery clock together with the ack deadline.
	LeasedAt  *time.Time
	AckedAt   *time.Time
	CreatedAt time.Time `gorm:"index;not null"`
}
