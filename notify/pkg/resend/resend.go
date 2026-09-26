package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gaucho-racing/notify/notify/config"
)

// The official SDK flattens every non-429 failure into a bare string error,
// which loses the status code Notify needs to tell a bad recipient (never
// retry) from an outage or misconfiguration (retry), so this talks to the
// REST API directly.
const baseURL = "https://api.resend.com"

var httpClient = &http.Client{Timeout: 10 * time.Second}

type Email struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text,omitempty"`
	HTML    string   `json:"html,omitempty"`
}

type Error struct {
	StatusCode int
	Name       string `json:"name"`
	Message    string `json:"message"`
	RetryAfter time.Duration
}

func (e Error) Error() string {
	return fmt.Sprintf("resend error: [%d] %s: %s", e.StatusCode, e.Name, e.Message)
}

// Permanent reports whether retrying cannot succeed for this recipient. Only
// request validation failures qualify; auth, domain, and quota errors are
// deployment problems that resolve once an operator fixes them.
func (e Error) Permanent() bool {
	return e.StatusCode == http.StatusUnprocessableEntity || e.StatusCode == http.StatusBadRequest
}

func (e Error) RetryAfterDuration() time.Duration {
	return e.RetryAfter
}

// Send delivers one email. idempotencyKey makes a retry after a lost response
// return the original send instead of emailing the recipient twice; Resend
// honors it for 24 hours.
func Send(ctx context.Context, email Email, idempotencyKey string) (string, error) {
	if strings.TrimSpace(config.ResendAPIKey) == "" {
		return "", fmt.Errorf("RESEND_API_KEY is not configured")
	}
	body, err := json.Marshal(email)
	if err != nil {
		return "", fmt.Errorf("encode email: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+config.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		resendErr := Error{StatusCode: resp.StatusCode}
		if err := json.Unmarshal(respBody, &resendErr); err != nil || resendErr.Message == "" {
			resendErr.Message = strings.TrimSpace(string(respBody))
		}
		if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds > 0 {
			resendErr.RetryAfter = time.Duration(seconds) * time.Second
		}
		return "", resendErr
	}

	var sent struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &sent); err != nil {
		return "", fmt.Errorf("decode resend response: %w", err)
	}
	return sent.ID, nil
}
