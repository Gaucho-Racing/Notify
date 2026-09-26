package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gaucho-racing/notify/notify/pkg/logger"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
)

const identityCacheTTL = 5 * time.Minute
const identityResolveBatchSize = 100

type cachedIdentity struct {
	summary   sentinel.IdentitySummary
	fetchedAt time.Time
}

var (
	identitiesMu     sync.Mutex
	cachedIdentities = map[string]cachedIdentity{}
)

func ResolveIdentities(ctx context.Context, entityIDs []string) ([]sentinel.IdentitySummary, error) {
	entityIDs = uniqueIdentityIDs(entityIDs)
	if len(entityIDs) == 0 {
		return []sentinel.IdentitySummary{}, nil
	}

	identitiesMu.Lock()
	defer identitiesMu.Unlock()

	now := time.Now()
	missing := make([]string, 0, len(entityIDs))
	for _, entityID := range entityIDs {
		entry, exists := cachedIdentities[entityID]
		if !exists || now.Sub(entry.fetchedAt) >= identityCacheTTL {
			missing = append(missing, entityID)
		}
	}

	if len(missing) > 0 {
		for start := 0; start < len(missing); start += identityResolveBatchSize {
			end := min(start+identityResolveBatchSize, len(missing))
			resolved, err := sentinel.ResolveIdentities(ctx, missing[start:end])
			if err != nil {
				if summaries, complete := cachedIdentityResults(entityIDs); complete {
					logger.SugarLogger.Warnf("serving stale identity summaries, refresh failed: %v", err)
					return summaries, nil
				}
				return nil, err
			}
			for _, summary := range resolved {
				cachedIdentities[summary.ID] = cachedIdentity{summary: summary, fetchedAt: now}
			}
		}
	}

	summaries, _ := cachedIdentityResults(entityIDs)
	return summaries, nil
}

func uniqueIdentityIDs(entityIDs []string) []string {
	seen := make(map[string]struct{}, len(entityIDs))
	unique := make([]string, 0, len(entityIDs))
	for _, entityID := range entityIDs {
		entityID = strings.TrimSpace(entityID)
		if entityID == "" {
			continue
		}
		if _, exists := seen[entityID]; exists {
			continue
		}
		seen[entityID] = struct{}{}
		unique = append(unique, entityID)
	}
	return unique
}

func cachedIdentityResults(entityIDs []string) ([]sentinel.IdentitySummary, bool) {
	summaries := make([]sentinel.IdentitySummary, 0, len(entityIDs))
	complete := true
	for _, entityID := range entityIDs {
		entry, exists := cachedIdentities[entityID]
		if !exists {
			complete = false
			continue
		}
		summaries = append(summaries, entry.summary)
	}
	return summaries, complete
}
