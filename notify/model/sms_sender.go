package model

import "time"

// SMSSender is a sending identity on one Twilio account: either a single
// FromNumber or a MessagingServiceSID whose sender pool Twilio picks from.
// Exactly one of the two is set.
type SMSSender struct {
	ID                  string    `json:"id" gorm:"primaryKey"`
	Name                string    `json:"name" gorm:"uniqueIndex"`
	Description         string    `json:"description"`
	TwilioAccountID     string    `json:"twilio_account_id" gorm:"index"`
	FromNumber          string    `json:"from_number"`
	MessagingServiceSID string    `json:"messaging_service_sid"`
	IsDefault           bool      `json:"is_default"`
	Enabled             bool      `json:"enabled"`
	CreatedByEntityID   string    `json:"created_by_entity_id" gorm:"index"`
	UpdatedByEntityID   string    `json:"updated_by_entity_id" gorm:"index"`
	CreatedAt           time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SMSSender) TableName() string {
	return "notify_sms_sender"
}

func (s SMSSender) From() string {
	if s.FromNumber != "" {
		return s.FromNumber
	}
	return "Messaging Service " + s.MessagingServiceSID
}
