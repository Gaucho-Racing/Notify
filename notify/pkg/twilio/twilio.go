package twilio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

const (
	apiHost       = "https://api.twilio.com"
	messagingHost = "https://messaging.twilio.com"
	// MaxBodyLength is Twilio's cap on a message body, counted in UTF-16
	// code units; longer bodies are rejected with error 21617.
	MaxBodyLength = 1600
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

var e164 = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

// recipientErrorCodes are failures specific to the destination number, so
// retrying the same delivery cannot succeed.
var recipientErrorCodes = []int{
	21211, // invalid To number
	21408, // destination region not enabled
	21610, // recipient replied STOP
	21612, // To/From combination cannot be routed
	21614, // To is not a mobile number
	21617, // body too long
}

// Credentials authenticate as either the account itself (KeySID empty,
// Secret is the auth token) or an API key on it.
type Credentials struct {
	AccountSID string
	KeySID     string
	Secret     string
}

func (c Credentials) username() string {
	if c.KeySID != "" {
		return c.KeySID
	}
	return c.AccountSID
}

// Message sends from exactly one of From or MessagingServiceSID.
type Message struct {
	To                  string
	From                string
	MessagingServiceSID string
	Body                string
	// StatusCallback receives Twilio's delivery receipts for this message.
	StatusCallback string
}

type PhoneNumber struct {
	PhoneNumber  string `json:"phone_number"`
	FriendlyName string `json:"friendly_name"`
	SMS          bool   `json:"sms"`
}

type MessagingService struct {
	SID          string `json:"sid"`
	FriendlyName string `json:"friendly_name"`
}

type Error struct {
	StatusCode int
	Code       int    `json:"code"`
	Message    string `json:"message"`
	RetryAfter time.Duration
}

func (e Error) Error() string {
	return fmt.Sprintf("twilio error: [%d] %d: %s", e.StatusCode, e.Code, e.Message)
}

// Permanent is limited to recipient-specific codes. Credential, sender, and
// account errors resolve once an admin fixes the configuration.
func (e Error) Permanent() bool {
	return slices.Contains(recipientErrorCodes, e.Code)
}

func (e Error) RetryAfterDuration() time.Duration {
	return e.RetryAfter
}

func IsAuthError(err error) bool {
	var twilioErr Error
	return errors.As(err, &twilioErr) && (twilioErr.StatusCode == http.StatusUnauthorized || twilioErr.Code == 20003)
}

func IsNotFound(err error) bool {
	var twilioErr Error
	return errors.As(err, &twilioErr) && twilioErr.StatusCode == http.StatusNotFound
}

// NormalizePhoneNumber strips common formatting and accepts only E.164, the
// only form Twilio takes for To and From.
func NormalizePhoneNumber(value string) (string, bool) {
	normalized := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '(', ')', '.':
			return -1
		}
		return r
	}, strings.TrimSpace(value))
	return normalized, e164.MatchString(normalized)
}

// BodyLength counts text the way Twilio measures MaxBodyLength.
func BodyLength(text string) int {
	return len(utf16.Encode([]rune(text)))
}

// Send creates one message. Twilio has no idempotency key for this call, so
// a request whose response is lost can be sent twice on retry.
func Send(ctx context.Context, creds Credentials, message Message) (string, error) {
	form := url.Values{}
	form.Set("To", message.To)
	form.Set("Body", message.Body)
	if message.MessagingServiceSID != "" {
		form.Set("MessagingServiceSid", message.MessagingServiceSID)
	} else {
		form.Set("From", message.From)
	}
	if message.StatusCallback != "" {
		form.Set("StatusCallback", message.StatusCallback)
	}
	var created struct {
		SID string `json:"sid"`
	}
	path := "/2010-04-01/Accounts/" + url.PathEscape(creds.AccountSID) + "/Messages.json"
	if err := call(ctx, creds, http.MethodPost, apiHost+path, form, &created); err != nil {
		return "", err
	}
	return created.SID, nil
}

// ListPhoneNumbers doubles as the credential check: it needs the account SID
// and the key to agree.
func ListPhoneNumbers(ctx context.Context, creds Credentials) ([]PhoneNumber, error) {
	numbers := []PhoneNumber{}
	next := apiHost + "/2010-04-01/Accounts/" + url.PathEscape(creds.AccountSID) + "/IncomingPhoneNumbers.json?PageSize=100"
	for next != "" {
		var page struct {
			NextPageURI *string `json:"next_page_uri"`
			Numbers     []struct {
				PhoneNumber  string `json:"phone_number"`
				FriendlyName string `json:"friendly_name"`
				Capabilities struct {
					SMS bool `json:"sms"`
				} `json:"capabilities"`
			} `json:"incoming_phone_numbers"`
		}
		if err := call(ctx, creds, http.MethodGet, next, nil, &page); err != nil {
			return nil, err
		}
		for _, number := range page.Numbers {
			numbers = append(numbers, PhoneNumber{
				PhoneNumber:  number.PhoneNumber,
				FriendlyName: number.FriendlyName,
				SMS:          number.Capabilities.SMS,
			})
		}
		next = ""
		if page.NextPageURI != nil && strings.HasPrefix(*page.NextPageURI, "/") && len(page.Numbers) > 0 {
			next = apiHost + *page.NextPageURI
		}
	}
	return numbers, nil
}

func ListMessagingServices(ctx context.Context, creds Credentials) ([]MessagingService, error) {
	services := []MessagingService{}
	next := messagingHost + "/v1/Services?PageSize=100"
	for next != "" {
		var page struct {
			Services []MessagingService `json:"services"`
			Meta     struct {
				NextPageURL *string `json:"next_page_url"`
			} `json:"meta"`
		}
		if err := call(ctx, creds, http.MethodGet, next, nil, &page); err != nil {
			return nil, err
		}
		services = append(services, page.Services...)
		next = ""
		if page.Meta.NextPageURL != nil && strings.HasPrefix(*page.Meta.NextPageURL, messagingHost+"/") && len(page.Services) > 0 {
			next = *page.Meta.NextPageURL
		}
	}
	return services, nil
}

func GetMessagingService(ctx context.Context, creds Credentials, sid string) (MessagingService, error) {
	var service MessagingService
	err := call(ctx, creds, http.MethodGet, messagingHost+"/v1/Services/"+url.PathEscape(sid), nil, &service)
	return service, err
}

func call(ctx context.Context, creds Credentials, method string, endpoint string, form url.Values, out interface{}) error {
	if creds.AccountSID == "" || creds.Secret == "" {
		return fmt.Errorf("twilio credentials are incomplete")
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(creds.username(), creds.Secret)
	req.Header.Set("Accept", "application/json")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
		twilioErr := Error{StatusCode: resp.StatusCode}
		if err := json.Unmarshal(respBody, &twilioErr); err != nil || twilioErr.Message == "" {
			twilioErr.Message = strings.TrimSpace(string(respBody))
		}
		if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds > 0 {
			twilioErr.RetryAfter = time.Duration(seconds) * time.Second
		}
		return twilioErr
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode twilio response: %w", err)
	}
	return nil
}
