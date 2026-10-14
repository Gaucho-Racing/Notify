package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://discord.com/api/v10"

var httpClient = &http.Client{Timeout: 10 * time.Second}

const (
	codeUnknownUser       = 10013
	codeCannotMessageUser = 50007
	codeInvalidFormBody   = 50035
	codeInvalidRecipient  = 50033
)

type Embed struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	URL         string       `json:"url,omitempty"`
	Color       int          `json:"color,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
	Author      *EmbedAuthor `json:"author,omitempty"`
	Footer      *EmbedFooter `json:"footer,omitempty"`
	Image       *EmbedMedia  `json:"image,omitempty"`
	Thumbnail   *EmbedMedia  `json:"thumbnail,omitempty"`
	Fields      []EmbedField `json:"fields,omitempty"`
}

type EmbedAuthor struct {
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

type EmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

type EmbedMedia struct {
	URL string `json:"url"`
}

type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type Message struct {
	Content string  `json:"content,omitempty"`
	Embeds  []Embed `json:"embeds,omitempty"`
}

type Error struct {
	StatusCode int
	Code       int     `json:"code"`
	Message    string  `json:"message"`
	RetryAfter float64 `json:"retry_after"`
}

func (e Error) Error() string {
	return fmt.Sprintf("discord error: [%d] %d %s", e.StatusCode, e.Code, e.Message)
}

// Permanent is limited to failures specific to the recipient — DMs closed,
// unknown account — so a revoked bot token still retries until it is fixed.
func (e Error) Permanent() bool {
	switch e.Code {
	case codeUnknownUser, codeCannotMessageUser, codeInvalidRecipient, codeInvalidFormBody:
		return true
	}
	return false
}

func (e Error) RetryAfterDuration() time.Duration {
	return time.Duration(e.RetryAfter * float64(time.Second))
}

// SendDirectMessage opens (or reuses) the DM channel with a user and posts to
// it. Both calls are plain REST, so Notify needs no gateway connection; the
// bot only has to share a guild with the recipient.
func SendDirectMessage(ctx context.Context, token string, userID string, message Message) (string, error) {
	var channel struct {
		ID string `json:"id"`
	}
	if err := call(ctx, token, http.MethodPost, "/users/@me/channels", map[string]string{"recipient_id": userID}, &channel); err != nil {
		return "", err
	}

	var sent struct {
		ID string `json:"id"`
	}
	if err := call(ctx, token, http.MethodPost, "/channels/"+url.PathEscape(channel.ID)+"/messages", message, &sent); err != nil {
		return "", err
	}
	return sent.ID, nil
}

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Bot      bool   `json:"bot"`
}

func (u User) AvatarURL() string {
	if u.Avatar == "" {
		return ""
	}
	return "https://cdn.discordapp.com/avatars/" + url.PathEscape(u.ID) + "/" + url.PathEscape(u.Avatar) + ".png"
}

// GetCurrentUser identifies the account a token belongs to, which doubles as
// a check that the token is valid before Notify stores it.
func GetCurrentUser(ctx context.Context, token string) (User, error) {
	var user User
	err := call(ctx, token, http.MethodGet, "/users/@me", nil, &user)
	return user, err
}

func call(ctx context.Context, token string, method string, path string, payload interface{}, out interface{}) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("discord bot token is empty")
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode discord request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		discordErr := Error{StatusCode: resp.StatusCode}
		if err := json.Unmarshal(respBody, &discordErr); err != nil || discordErr.Message == "" {
			discordErr.Message = strings.TrimSpace(string(respBody))
		}
		return discordErr
	}
	return json.Unmarshal(respBody, out)
}
