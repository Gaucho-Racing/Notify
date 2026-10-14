package model

import "time"

// TwilioAccount is one set of Twilio credentials Notify can send SMS through.
// AccountSID is fixed once created, since senders' numbers belong to that
// account. KeySID is empty when Secret is the account's auth token rather
// than an API key secret.
type TwilioAccount struct {
	ID                string    `json:"id" gorm:"primaryKey"`
	Name              string    `json:"name" gorm:"uniqueIndex"`
	Description       string    `json:"description"`
	AccountSID        string    `json:"account_sid"`
	KeySID            string    `json:"key_sid"`
	SecretCiphertext  []byte    `json:"-"`
	SecretHint        string    `json:"secret_hint"`
	Enabled           bool      `json:"enabled"`
	CreatedByEntityID string    `json:"created_by_entity_id" gorm:"index"`
	UpdatedByEntityID string    `json:"updated_by_entity_id" gorm:"index"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TwilioAccount) TableName() string {
	return "notify_twilio_account"
}
