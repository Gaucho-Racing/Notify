package discord

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gaucho-racing/notify/notify/pkg/logger"
)

const (
	maxWebhookUsername   = 80
	webhookName          = "Notify"
	webhookCleanupWindow = 10 * time.Second
)

// WebhookIdentity is the name and avatar a channel post appears under in
// place of the bot's own.
type WebhookIdentity struct {
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type webhookMessage struct {
	Message
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// ValidateWebhookIdentity applies Discord's rules for webhook usernames, which
// follow its nickname rules with an 80-character limit.
func ValidateWebhookIdentity(identity WebhookIdentity) error {
	name := identity.Username
	lower := strings.ToLower(name)
	switch {
	case strings.TrimSpace(name) == "":
		return errors.New("webhook.username is required")
	case utf8.RuneCountInString(name) > maxWebhookUsername:
		return fmt.Errorf("webhook.username must be at most %d characters", maxWebhookUsername)
	case strings.Contains(lower, "discord"), strings.Contains(lower, "clyde"):
		return errors.New(`webhook.username cannot contain "discord" or "clyde"`)
	case strings.ContainsAny(name, "@#:") || strings.Contains(name, "```"):
		return errors.New("webhook.username cannot contain @, #, :, or ```")
	case lower == "everyone" || lower == "here":
		return errors.New(`webhook.username cannot be "everyone" or "here"`)
	case identity.AvatarURL != "" && !isHTTPURL(identity.AvatarURL):
		return errors.New("webhook.avatar_url must be an absolute http(s) URL")
	}
	return nil
}

// SendAsWebhook posts to a channel under a custom name and avatar by creating
// a webhook, executing it, and deleting it again, so no long-lived webhook
// credential exists. The bot needs Manage Webhooks in the channel. Cleanup
// runs on its own deadline so a send that used up ctx still deletes the
// webhook; a failed cleanup is logged rather than failing a message that was
// already posted.
func SendAsWebhook(ctx context.Context, token string, channelID string, identity WebhookIdentity, message Message) (string, error) {
	var webhook struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if err := call(ctx, token, http.MethodPost, "/channels/"+url.PathEscape(channelID)+"/webhooks", map[string]string{"name": webhookName}, &webhook); err != nil {
		return "", err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), webhookCleanupWindow)
		defer cancel()
		if err := call(cleanupCtx, token, http.MethodDelete, "/webhooks/"+url.PathEscape(webhook.ID), nil, nil); err != nil {
			logger.SugarLogger.Warnf("failed to delete temporary discord webhook %s in channel %s: %v", webhook.ID, channelID, err)
		}
	}()

	var sent struct {
		ID string `json:"id"`
	}
	path := "/webhooks/" + url.PathEscape(webhook.ID) + "/" + url.PathEscape(webhook.Token) + "?wait=true"
	payload := webhookMessage{Message: message, Username: identity.Username, AvatarURL: identity.AvatarURL}
	if err := request(ctx, "", http.MethodPost, path, payload, &sent); err != nil {
		return "", err
	}
	return sent.ID, nil
}
