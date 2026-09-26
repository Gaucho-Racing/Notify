package model

import "time"

// AppPolicy authorizes one application (by Sentinel client_id) to send
// through Notify. Applications without a policy cannot send at all, so every
// channel an app reaches people on is an explicit admin decision.
type AppPolicy struct {
	ID              string    `json:"id" gorm:"primaryKey"`
	ClientID        string    `json:"client_id" gorm:"uniqueIndex"`
	Description     string    `json:"description"`
	AllowedChannels []Channel `json:"allowed_channels" gorm:"serializer:json;type:jsonb"`
	CanTargetGroups bool      `json:"can_target_groups"`
	// DailyLimit caps deliveries created in a rolling 24 hours; 0 means
	// unlimited.
	DailyLimit        int       `json:"daily_limit"`
	Enabled           bool      `json:"enabled"`
	CreatedByEntityID string    `json:"created_by_entity_id" gorm:"index"`
	UpdatedByEntityID string    `json:"updated_by_entity_id" gorm:"index"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (AppPolicy) TableName() string {
	return "notify_app_policy"
}

func (p AppPolicy) AllowsChannel(channel Channel) bool {
	for _, allowed := range p.AllowedChannels {
		if allowed == channel {
			return true
		}
	}
	return false
}
