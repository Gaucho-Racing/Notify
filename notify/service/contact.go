package service

import (
	"context"
	"errors"
	"net/http"

	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
)

var ErrRecipientNotFound = errors.New("recipient not found in Sentinel")

// GetContact resolves where an entity can be reached at delivery time. It is
// deliberately uncached: a stale address would send codes for a just-changed
// email to the previous one.
func GetContact(ctx context.Context, entityID string) (sentinel.Contact, error) {
	contact, err := sentinel.GetContact(ctx, entityID)
	if err != nil {
		var sentinelErr sentinel.Error
		if errors.As(err, &sentinelErr) && sentinelErr.Code == http.StatusNotFound {
			return sentinel.Contact{}, ErrRecipientNotFound
		}
		return sentinel.Contact{}, err
	}
	return contact, nil
}
