package service

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
)

const contactCacheTTL = 5 * time.Minute

var ErrRecipientNotFound = errors.New("recipient not found in Sentinel")

type cachedContact struct {
	contact   sentinel.Contact
	fetchedAt time.Time
}

var (
	contactsMu     sync.Mutex
	cachedContacts = map[string]cachedContact{}
)

// GetContact resolves where an entity can be reached. Sentinel has no batch
// contact endpoint, so a group send costs one lookup per member; the cache
// keeps a burst of messages to the same people from repeating them.
func GetContact(ctx context.Context, entityID string) (sentinel.Contact, error) {
	contactsMu.Lock()
	entry, exists := cachedContacts[entityID]
	contactsMu.Unlock()
	if exists && time.Since(entry.fetchedAt) < contactCacheTTL {
		return entry.contact, nil
	}

	contact, err := sentinel.GetContact(ctx, entityID)
	if err != nil {
		var sentinelErr sentinel.Error
		if errors.As(err, &sentinelErr) && sentinelErr.Code == http.StatusNotFound {
			return sentinel.Contact{}, ErrRecipientNotFound
		}
		return sentinel.Contact{}, err
	}

	contactsMu.Lock()
	cachedContacts[entityID] = cachedContact{contact: contact, fetchedAt: time.Now()}
	contactsMu.Unlock()
	return contact, nil
}
