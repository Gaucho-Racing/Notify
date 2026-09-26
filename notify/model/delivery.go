package model

import "time"

type DeliveryStatus string

const (
	DeliveryStatusPending DeliveryStatus = "PENDING"
	DeliveryStatusSending DeliveryStatus = "SENDING"
	DeliveryStatusSent    DeliveryStatus = "SENT"
	DeliveryStatusFailed  DeliveryStatus = "FAILED"
	// Skipped deliveries were never attempted with a provider, e.g. the
	// recipient has no address on file for the channel.
	DeliveryStatusSkipped DeliveryStatus = "SKIPPED"
)

func (s DeliveryStatus) Valid() bool {
	switch s {
	case DeliveryStatusPending, DeliveryStatusSending, DeliveryStatusSent, DeliveryStatusFailed, DeliveryStatusSkipped:
		return true
	}
	return false
}

// Delivery is one attempt to reach one recipient on one channel. The table is
// also the work queue: workers claim PENDING rows whose NextAttemptAt has
// passed.
type Delivery struct {
	ID                string         `json:"id" gorm:"primaryKey"`
	MessageID         string         `json:"message_id" gorm:"index"`
	ClientID          string         `json:"client_id" gorm:"index"`
	EntityID          string         `json:"entity_id" gorm:"index"`
	Channel           Channel        `json:"channel" gorm:"index"`
	Provider          string         `json:"provider"`
	Destination       string         `json:"destination"`
	Status            DeliveryStatus `json:"status" gorm:"index"`
	Attempts          int            `json:"attempts"`
	NextAttemptAt     time.Time      `json:"next_attempt_at"`
	LockedAt          *time.Time     `json:"-"`
	LastError         string         `json:"last_error,omitempty"`
	ProviderMessageID string         `json:"provider_message_id,omitempty"`
	SentAt            *time.Time     `json:"sent_at,omitempty"`
	CreatedAt         time.Time      `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt         time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Delivery) TableName() string {
	return "notify_delivery"
}
