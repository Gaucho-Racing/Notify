package service

import (
	"strings"
	"sync"
	"time"

	"github.com/gaucho-racing/notify/notify/pkg/logger"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
)

const applicationCacheTTL = 5 * time.Minute

var (
	applicationsMu      sync.Mutex
	cachedApplications  []sentinel.Application
	applicationsFetched time.Time
)

// ListApplications returns Sentinel's application list, cached in process.
// Every caller gets the same answer — the response is authorized by Notify, not
// per user — so one fetch serves everyone. A refresh failure falls back to the
// last known good list rather than breaking the policy pickers over a blip in
// Sentinel.
func ListApplications() ([]sentinel.Application, error) {
	applicationsMu.Lock()
	defer applicationsMu.Unlock()

	if cachedApplications != nil && time.Since(applicationsFetched) < applicationCacheTTL {
		return cachedApplications, nil
	}

	applications, err := sentinel.GetApplications()
	if err != nil {
		if cachedApplications != nil {
			logger.SugarLogger.Warnf("serving stale application list, refresh failed: %v", err)
			return cachedApplications, nil
		}
		return nil, err
	}
	cachedApplications = applications
	applicationsFetched = time.Now()
	return applications, nil
}

func ResolveApplications(clientIDs []string) ([]sentinel.Application, error) {
	clientIDs = uniqueClientIDs(clientIDs)
	if len(clientIDs) == 0 {
		return []sentinel.Application{}, nil
	}
	applications, err := ListApplications()
	if err != nil {
		return nil, err
	}
	byClientID := make(map[string]sentinel.Application, len(applications))
	for _, application := range applications {
		byClientID[application.ClientID] = application
	}
	resolved := make([]sentinel.Application, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		if application, exists := byClientID[clientID]; exists {
			resolved = append(resolved, application)
		}
	}
	return resolved, nil
}

func uniqueClientIDs(clientIDs []string) []string {
	seen := make(map[string]struct{}, len(clientIDs))
	unique := make([]string, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		clientID = strings.TrimSpace(clientID)
		if clientID == "" {
			continue
		}
		if _, exists := seen[clientID]; exists {
			continue
		}
		seen[clientID] = struct{}{}
		unique = append(unique, clientID)
	}
	return unique
}
