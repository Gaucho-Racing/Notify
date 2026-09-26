package sentinel

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

	"github.com/gaucho-racing/notify/notify/config"
	"github.com/golang-jwt/jwt/v5"
)

type Error struct {
	Code    int
	Message string `json:"error"`
}

func (e Error) Error() string {
	if e.Code == 0 {
		return e.Message
	}
	return fmt.Sprintf("sentinel error: [%d] %s", e.Code, e.Message)
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type User struct {
	ID          string   `json:"id"`
	EntityID    string   `json:"entity_id"`
	Username    string   `json:"username"`
	FirstName   string   `json:"first_name"`
	LastName    string   `json:"last_name"`
	Email       string   `json:"email"`
	AvatarURL   string   `json:"avatar_url"`
	InitialRole string   `json:"initial_role"`
	Groups      []string `json:"groups"`
	UpdatedAt   string   `json:"updated_at"`
	CreatedAt   string   `json:"created_at"`
}

type Application struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ClientID    string `json:"client_id"`
	IconURL     string `json:"icon_url"`
}

type IdentityApplicationSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ClientID string `json:"client_id"`
	IconURL  string `json:"icon_url"`
}

type IdentitySummary struct {
	ID          string                      `json:"id"`
	Type        string                      `json:"type"`
	Name        string                      `json:"name"`
	Username    string                      `json:"username,omitempty"`
	AvatarURL   string                      `json:"avatar_url,omitempty"`
	Application *IdentityApplicationSummary `json:"application,omitempty"`
}

type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MemberCount int64  `json:"member_count"`
}

var httpClient = &http.Client{Timeout: 5 * time.Second}

// Token exchange happens once per sign-in and its failure lands the user on an
// error screen, so it tolerates a slow Sentinel rather than failing fast.
var authClient = &http.Client{Timeout: 20 * time.Second}

// ValidateToken verifies a Sentinel JWT locally against the JWKS loaded at
// boot: RS256 signature, the registered time claims, and a non-empty audience,
// which mirrors what Sentinel's own validate endpoint enforces.
//
// The one check it cannot reproduce is revocation — Sentinel confirms the
// token's jti still has an auth_token row, which is how rotating or deleting a
// service account kills its outstanding tokens. Accepted deliberately: a
// request per call to an external service on Notify's hottest path is a worse
// trade, and a revoked token here stays usable until it expires.
func ValidateToken(token string) (map[string]interface{}, error) {
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		return lookupKey(kid)
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("token is invalid")
	}
	if audience, ok := claims["aud"]; !ok || audience == nil {
		return nil, fmt.Errorf("token has invalid audience")
	}
	return claims, nil
}

func ExchangeAuthorizationCode(code string, redirectURI string) (TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	return exchangeToken(form)
}

func RefreshToken(refreshToken string) (TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	return exchangeToken(form)
}

func GetCurrentUser(accessToken string, userID string) (User, error) {
	if strings.TrimSpace(config.SentinelURL) == "" {
		return User{}, fmt.Errorf("SENTINEL_URL is not configured")
	}
	if strings.TrimSpace(accessToken) == "" {
		return User{}, fmt.Errorf("access token is required")
	}
	if strings.TrimSpace(userID) == "" {
		return User{}, fmt.Errorf("user id is required")
	}

	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(config.SentinelURL, "/")+"/api/users/"+url.PathEscape(userID), nil)
	if err != nil {
		return User{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return User{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return User{}, err
	}
	if resp.StatusCode != http.StatusOK {
		var sentinelErr Error
		if err := json.Unmarshal(respBody, &sentinelErr); err == nil && sentinelErr.Message != "" {
			sentinelErr.Code = resp.StatusCode
			return User{}, sentinelErr
		}
		return User{}, Error{Code: resp.StatusCode, Message: strings.TrimSpace(string(respBody))}
	}

	var user User
	if err := json.Unmarshal(respBody, &user); err != nil {
		return User{}, err
	}
	return user, nil
}

func GetGroups(accessToken string) ([]Group, error) {
	if strings.TrimSpace(config.SentinelURL) == "" {
		return []Group{}, fmt.Errorf("SENTINEL_URL is not configured")
	}
	if strings.TrimSpace(accessToken) == "" {
		return []Group{}, fmt.Errorf("access token is required")
	}

	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(config.SentinelURL, "/")+"/api/groups", nil)
	if err != nil {
		return []Group{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return []Group{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return []Group{}, err
	}
	if resp.StatusCode != http.StatusOK {
		var sentinelErr Error
		if err := json.Unmarshal(respBody, &sentinelErr); err == nil && sentinelErr.Message != "" {
			sentinelErr.Code = resp.StatusCode
			return []Group{}, sentinelErr
		}
		return []Group{}, Error{Code: resp.StatusCode, Message: strings.TrimSpace(string(respBody))}
	}

	groups := []Group{}
	if err := json.Unmarshal(respBody, &groups); err != nil {
		return []Group{}, err
	}
	return groups, nil
}

// GetApplications lists every Sentinel application using Notify's own service
// account token rather than the caller's. Notify already decides who may see
// the list (admins only), so forwarding a user token would just mean every
// human session had to carry applications:read.
func GetApplications() ([]Application, error) {
	if strings.TrimSpace(config.SentinelURL) == "" {
		return []Application{}, fmt.Errorf("SENTINEL_URL is not configured")
	}
	accessToken := strings.TrimSpace(config.SentinelSAToken)
	if accessToken == "" {
		return []Application{}, fmt.Errorf("SENTINEL_SA_TOKEN is not configured")
	}

	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(config.SentinelURL, "/")+"/api/applications", nil)
	if err != nil {
		return []Application{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return []Application{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return []Application{}, err
	}
	if resp.StatusCode != http.StatusOK {
		var sentinelErr Error
		if err := json.Unmarshal(respBody, &sentinelErr); err == nil && sentinelErr.Message != "" {
			sentinelErr.Code = resp.StatusCode
			return []Application{}, sentinelErr
		}
		return []Application{}, Error{Code: resp.StatusCode, Message: strings.TrimSpace(string(respBody))}
	}

	applications := []Application{}
	if err := json.Unmarshal(respBody, &applications); err != nil {
		return []Application{}, err
	}
	return applications, nil
}

func ResolveIdentities(ctx context.Context, entityIDs []string) ([]IdentitySummary, error) {
	if strings.TrimSpace(config.SentinelURL) == "" {
		return nil, fmt.Errorf("SENTINEL_URL is not configured")
	}
	accessToken := strings.TrimSpace(config.SentinelSAToken)
	if accessToken == "" {
		return nil, fmt.Errorf("SENTINEL_SA_TOKEN is not configured")
	}

	requestBody, err := json.Marshal(map[string][]string{"ids": entityIDs})
	if err != nil {
		return nil, fmt.Errorf("encode identity summary request: %w", err)
	}
	requestURL := strings.TrimRight(config.SentinelURL, "/") + "/api/entities/resolve"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var sentinelErr Error
		if err := json.Unmarshal(respBody, &sentinelErr); err == nil && sentinelErr.Message != "" {
			sentinelErr.Code = resp.StatusCode
			return nil, sentinelErr
		}
		return nil, Error{Code: resp.StatusCode, Message: strings.TrimSpace(string(respBody))}
	}

	summaries := []IdentitySummary{}
	if err := json.Unmarshal(respBody, &summaries); err != nil {
		return nil, err
	}
	return summaries, nil
}

func exchangeToken(form url.Values) (TokenResponse, error) {
	if strings.TrimSpace(config.SentinelURL) == "" {
		return TokenResponse{}, fmt.Errorf("SENTINEL_URL is not configured")
	}
	if strings.TrimSpace(config.SentinelClientID) == "" {
		return TokenResponse{}, fmt.Errorf("SENTINEL_CLIENT_ID is not configured")
	}
	if strings.TrimSpace(config.SentinelClientSecret) == "" {
		return TokenResponse{}, fmt.Errorf("SENTINEL_CLIENT_SECRET is not configured")
	}
	form.Set("client_id", config.SentinelClientID)
	form.Set("client_secret", config.SentinelClientSecret)

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(config.SentinelURL, "/")+"/api/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := authClient.Do(req)
	if err != nil {
		return TokenResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenResponse{}, err
	}
	if resp.StatusCode != http.StatusOK {
		var sentinelErr Error
		if err := json.Unmarshal(respBody, &sentinelErr); err != nil {
			return TokenResponse{}, err
		}
		sentinelErr.Code = resp.StatusCode
		return TokenResponse{}, fmt.Errorf("sentinel error: [%d] %s", sentinelErr.Code, sentinelErr.Message)
	}

	var token TokenResponse
	if err := json.Unmarshal(respBody, &token); err != nil {
		return TokenResponse{}, err
	}
	if token.AccessToken == "" {
		return TokenResponse{}, fmt.Errorf("sentinel token response did not include access token")
	}
	return token, nil
}
