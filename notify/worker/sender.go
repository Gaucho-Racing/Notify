package worker

import (
	"context"
	"time"

	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
)

// Outbound is everything a channel needs to render and send one delivery.
type Outbound struct {
	DeliveryID      string
	Message         model.Message
	Contact         sentinel.Contact
	ApplicationName string
}

type Sender interface {
	Provider() string
	// Destination returns the recipient's address on this channel, or false
	// when they have none on file.
	Destination(contact sentinel.Contact) (string, bool)
	// MaskDestination is the form stored on the delivery row for display.
	MaskDestination(destination string) string
	Send(ctx context.Context, outbound Outbound, destination string) (providerMessageID string, err error)
}

// permanentError and retryAfterError are satisfied by provider errors so the
// worker can classify failures without knowing each provider's codes.
type permanentError interface {
	Permanent() bool
}

type retryAfterError interface {
	RetryAfterDuration() time.Duration
}

var senders = map[model.Channel]Sender{
	model.ChannelEmail:   emailSender{},
	model.ChannelDiscord: discordSender{},
	model.ChannelSMS:     smsSender{},
}

// unavailableError marks a delivery whose bot or sender is gone or disabled.
// Retrying cannot help until an admin intervenes, and a manual retry re-reads
// the configuration anyway.
type unavailableError struct {
	error
}

func (unavailableError) Permanent() bool {
	return true
}
