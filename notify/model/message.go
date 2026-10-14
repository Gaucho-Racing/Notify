package model

import (
	"time"

	"github.com/gaucho-racing/notify/notify/pkg/discord"
)

// MessageRecipients names people (directly or through groups) and, for
// Discord, guild channels to post in.
type MessageRecipients struct {
	EntityIDs         []string `json:"entity_ids"`
	GroupIDs          []string `json:"group_ids"`
	DiscordChannelIDs []string `json:"discord_channel_ids,omitempty"`
}

// Message is one send request as the calling application made it. Group
// recipients are expanded into per-entity deliveries at send time, so
// Recipients preserves what was asked for while RecipientCount records who it
// resolved to.
type Message struct {
	ID             string    `json:"id" gorm:"primaryKey"`
	ClientID       string    `json:"client_id" gorm:"index"`
	SenderEntityID string    `json:"sender_entity_id" gorm:"index"`
	SenderType     ActorType `json:"sender_type"`
	Subject        string    `json:"subject"`
	Body           string    `json:"body"`
	// Body is the email's plain text, the SMS, or the Discord message
	// content. EmailHTML and DiscordEmbeds are the rich versions.
	EmailHTML     string          `json:"email_html,omitempty" gorm:"column:html"`
	DiscordEmbeds []discord.Embed `json:"discord_embeds,omitempty" gorm:"serializer:json;type:jsonb"`
	Category      string          `json:"category,omitempty" gorm:"index"`
	Metadata      map[string]any  `json:"metadata" gorm:"serializer:json;type:jsonb"`
	Channel       Channel         `json:"channel" gorm:"index"`
	// Only the sender for Channel is set. It is resolved at send time,
	// falling back to the default, so retries go out the same way even if the
	// default changes.
	DiscordBotID   string            `json:"discord_bot_id,omitempty" gorm:"index"`
	EmailSenderID  string            `json:"email_sender_id,omitempty" gorm:"index"`
	SMSSenderID    string            `json:"sms_sender_id,omitempty" gorm:"index"`
	Recipients     MessageRecipients `json:"recipients" gorm:"serializer:json;type:jsonb"`
	RecipientCount int               `json:"recipient_count"`
	CreatedAt      time.Time         `json:"created_at" gorm:"autoCreateTime;index"`
}

func (Message) TableName() string {
	return "notify_message"
}
