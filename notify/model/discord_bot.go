package model

import "time"

// DiscordBot is a bot account Notify can send Discord DMs through. The token
// is stored AES-GCM encrypted and never leaves the server; TokenHint is the
// last few characters so admins can tell which token is loaded.
type DiscordBot struct {
	ID                string    `json:"id" gorm:"primaryKey"`
	Name              string    `json:"name" gorm:"uniqueIndex"`
	Description       string    `json:"description"`
	DiscordUserID     string    `json:"discord_user_id"`
	DiscordUsername   string    `json:"discord_username"`
	AvatarURL         string    `json:"avatar_url"`
	TokenCiphertext   []byte    `json:"-"`
	TokenHint         string    `json:"token_hint"`
	IsDefault         bool      `json:"is_default"`
	Enabled           bool      `json:"enabled"`
	CreatedByEntityID string    `json:"created_by_entity_id" gorm:"index"`
	UpdatedByEntityID string    `json:"updated_by_entity_id" gorm:"index"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DiscordBot) TableName() string {
	return "notify_discord_bot"
}
