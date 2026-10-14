package model

import (
	"net/mail"
	"time"
)

// EmailSender is a From identity Notify can send email as, through one
// Resend account. FromAddress must be on a domain verified in that account;
// ReplyTo routes replies somewhere that
// actually receives mail, since the sending subdomain usually does not.
type EmailSender struct {
	ID                string    `json:"id" gorm:"primaryKey"`
	Name              string    `json:"name" gorm:"uniqueIndex"`
	Description       string    `json:"description"`
	ResendAccountID   string    `json:"resend_account_id" gorm:"index"`
	FromName          string    `json:"from_name"`
	FromAddress       string    `json:"from_address"`
	ReplyTo           string    `json:"reply_to"`
	IsDefault         bool      `json:"is_default"`
	Enabled           bool      `json:"enabled"`
	CreatedByEntityID string    `json:"created_by_entity_id" gorm:"index"`
	UpdatedByEntityID string    `json:"updated_by_entity_id" gorm:"index"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EmailSender) TableName() string {
	return "notify_email_sender"
}

func (s EmailSender) From() string {
	return (&mail.Address{Name: s.FromName, Address: s.FromAddress}).String()
}
