package sentinel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gaucho-racing/notify/notify/config"
)

// Contact is the subset of a Sentinel entity Notify needs to reach someone.
// The entity response also carries credentials for linked providers; decoding
// into this narrow shape drops them on the floor instead of holding them in
// memory or caches.
type Contact struct {
	EntityID    string
	Type        string
	Name        string
	Email       string
	PhoneNumber string
	DiscordID   string
}

type entityResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	EmailAuth struct {
		Email string `json:"email"`
	} `json:"email_auth"`
	PhoneAuth struct {
		PhoneNumber string `json:"phone_number"`
	} `json:"phone_auth"`
	ExternalAuths []struct {
		Provider   string `json:"provider"`
		ExternalID string `json:"external_id"`
	} `json:"external_auths"`
	User *struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
	} `json:"user"`
}

type GroupMember struct {
	GroupID  string `json:"group_id"`
	EntityID string `json:"entity_id"`
}

func GetContact(ctx context.Context, entityID string) (Contact, error) {
	var entity entityResponse
	if err := serviceAccountGet(ctx, "/api/entities/"+url.PathEscape(entityID), &entity); err != nil {
		return Contact{}, err
	}

	contact := Contact{
		EntityID:    entity.ID,
		Type:        entity.Type,
		Email:       strings.TrimSpace(entity.EmailAuth.Email),
		PhoneNumber: strings.TrimSpace(entity.PhoneAuth.PhoneNumber),
	}
	for _, auth := range entity.ExternalAuths {
		if auth.Provider == "DISCORD" {
			contact.DiscordID = strings.TrimSpace(auth.ExternalID)
		}
	}
	if entity.User != nil {
		contact.Name = strings.TrimSpace(entity.User.FirstName + " " + entity.User.LastName)
		if contact.Name == "" {
			contact.Name = entity.User.Username
		}
	}
	return contact, nil
}

func GetGroupMembers(ctx context.Context, groupID string) ([]GroupMember, error) {
	members := []GroupMember{}
	if err := serviceAccountGet(ctx, "/api/groups/"+url.PathEscape(groupID)+"/members", &members); err != nil {
		return nil, err
	}
	return members, nil
}

func serviceAccountGet(ctx context.Context, path string, out interface{}) error {
	if strings.TrimSpace(config.SentinelURL) == "" {
		return fmt.Errorf("SENTINEL_URL is not configured")
	}
	accessToken := strings.TrimSpace(config.SentinelSAToken)
	if accessToken == "" {
		return fmt.Errorf("SENTINEL_SA_TOKEN is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(config.SentinelURL, "/")+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		var sentinelErr Error
		if err := json.Unmarshal(respBody, &sentinelErr); err == nil && sentinelErr.Message != "" {
			sentinelErr.Code = resp.StatusCode
			return sentinelErr
		}
		return Error{Code: resp.StatusCode, Message: strings.TrimSpace(string(respBody))}
	}
	return json.Unmarshal(respBody, out)
}
