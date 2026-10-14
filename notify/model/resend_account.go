package model

import "time"

// ResendAccount is one Resend API key Notify can send email through. The key
// is stored AES-GCM encrypted and never leaves the server. FullAccess records
// whether the key can list domains, which is what lets Notify check a
// sender's domain before saving it.
type ResendAccount struct {
	ID                string    `json:"id" gorm:"primaryKey"`
	Name              string    `json:"name" gorm:"uniqueIndex"`
	Description       string    `json:"description"`
	KeyCiphertext     []byte    `json:"-"`
	KeyHint           string    `json:"key_hint"`
	FullAccess        bool      `json:"full_access"`
	Enabled           bool      `json:"enabled"`
	CreatedByEntityID string    `json:"created_by_entity_id" gorm:"index"`
	UpdatedByEntityID string    `json:"updated_by_entity_id" gorm:"index"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ResendAccount) TableName() string {
	return "notify_resend_account"
}
