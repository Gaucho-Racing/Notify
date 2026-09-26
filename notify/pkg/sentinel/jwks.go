package sentinel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gaucho-racing/notify/notify/config"
	"github.com/gaucho-racing/notify/notify/pkg/logger"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

// Sentinel's signing keys, fetched once at boot so token validation is a local
// signature check rather than a request per call.
var (
	keySetMu    sync.RWMutex
	keySet      jwk.Set
	lastFetched time.Time
)

// refetchFloor keeps a token signed by an unknown key from turning every
// request into a JWKS fetch. Key rotation and a Sentinel outage during boot
// both resolve within this window.
const refetchFloor = time.Minute

// InitializeKeys loads Sentinel's JWKS. A failure here is not fatal: the
// validation path refetches on demand, so Notify can start while Sentinel is
// briefly unavailable and recover once it returns.
func InitializeKeys() {
	if err := fetchKeySet(); err != nil {
		logger.SugarLogger.Errorf("Failed to load Sentinel JWKS, will retry on first token: %v", err)
		return
	}
	keySetMu.RLock()
	defer keySetMu.RUnlock()
	logger.SugarLogger.Infof("Loaded %d signing key(s) from Sentinel JWKS", keySet.Len())
}

func jwksURL() (string, error) {
	if strings.TrimSpace(config.SentinelURL) == "" {
		return "", fmt.Errorf("SENTINEL_URL is not configured")
	}
	return strings.TrimRight(config.SentinelURL, "/") + "/api/core/keys", nil
}

func fetchKeySet() error {
	url, err := jwksURL()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	set, err := jwk.Fetch(ctx, url)
	if err != nil {
		return err
	}
	if set.Len() == 0 {
		return fmt.Errorf("no keys found in Sentinel JWKS")
	}

	keySetMu.Lock()
	defer keySetMu.Unlock()
	keySet = set
	lastFetched = time.Now()
	return nil
}

// lookupKey resolves the token's `kid` against the cached key set, refetching
// once if the key is unknown. Sentinel stamps a kid on every token; the
// fallback to the first key covers a set that has not been rotated.
func lookupKey(kid string) (interface{}, error) {
	key, found := keyFromSet(kid)
	if !found {
		keySetMu.RLock()
		stale := time.Since(lastFetched) > refetchFloor || keySet == nil
		keySetMu.RUnlock()
		if stale {
			if err := fetchKeySet(); err != nil {
				return nil, fmt.Errorf("signing key %q unavailable: %w", kid, err)
			}
			key, found = keyFromSet(kid)
		}
	}
	if !found {
		return nil, fmt.Errorf("no signing key matches kid %q", kid)
	}

	var raw interface{}
	if err := key.Raw(&raw); err != nil {
		return nil, fmt.Errorf("decode signing key: %w", err)
	}
	return raw, nil
}

func keyFromSet(kid string) (jwk.Key, bool) {
	keySetMu.RLock()
	defer keySetMu.RUnlock()
	if keySet == nil || keySet.Len() == 0 {
		return nil, false
	}
	if kid != "" {
		if key, ok := keySet.LookupKeyID(kid); ok {
			return key, true
		}
		return nil, false
	}
	return keySet.Key(0)
}
