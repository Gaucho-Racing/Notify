package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	ReplyTo []string `json:"reply_to,omitempty"`
}

type Domain struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// CanSend covers partially_verified too: Resend still sends from those
// domains, just without a fallback sending server.
func (d Domain) CanSend() bool {
	return d.Status == "verified" || d.Status == "partially_verified"
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
	headers := map[string]string{}
	if idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}
	var sent struct {
		ID string `json:"id"`
	}
	if err := call(ctx, http.MethodPost, "/emails", email, headers, &sent); err != nil {
		return "", err
	}
	return sent.ID, nil
}

// ListDomains returns every domain on the account. A sending-only API key
// cannot call this; Resend answers with a restricted_api_key error, which
// IsRestrictedKey detects.
func ListDomains(ctx context.Context) ([]Domain, error) {
	domains := []Domain{}
	after := ""
	for {
		path := "/domains?limit=100"
		if after != "" {
			path += "&after=" + url.QueryEscape(after)
		}
		var page struct {
			HasMore bool `json:"has_more"`
			Data    []struct {
				ID string `json:"id"`
				Domain
			} `json:"data"`
		}
		if err := call(ctx, http.MethodGet, path, nil, nil, &page); err != nil {
			return nil, err
		}
		for _, domain := range page.Data {
			domains = append(domains, domain.Domain)
		}
		if !page.HasMore || len(page.Data) == 0 {
			return domains, nil
		}
		after = page.Data[len(page.Data)-1].ID
	}
}

func IsRestrictedKey(err error) bool {
	var resendErr Error
	return errors.As(err, &resendErr) && resendErr.Name == "restricted_api_key"
}

func call(ctx context.Context, method string, path string, payload interface{}, headers map[string]string, out interface{}) error {
	if strings.TrimSpace(config.ResendAPIKey) == "" {
		return fmt.Errorf("RESEND_API_KEY is not configured")
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode resend request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+config.ResendAPIKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
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
		resendErr := Error{StatusCode: resp.StatusCode}
		if err := json.Unmarshal(respBody, &resendErr); err != nil || resendErr.Message == "" {
			resendErr.Message = strings.TrimSpace(string(respBody))
		}
		if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds > 0 {
			resendErr.RetryAfter = time.Duration(seconds) * time.Second
		}
		return resendErr
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode resend response: %w", err)
	}
	return nil
}
