package worker

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/gaucho-racing/notify/notify/config"
	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/logger"
	"github.com/gaucho-racing/notify/notify/service"
)

const (
	pollInterval    = time.Second
	sendTimeout     = 30 * time.Second
	staleLockAfter  = 5 * time.Minute
	reapInterval    = time.Minute
	baseBackoff     = 30 * time.Second
	maxBackoff      = time.Hour
	maxRetryAfter   = time.Hour
	claimMultiplier = 2
)

// Start runs the dispatcher until ctx is cancelled, then waits for in-flight
// sends to finish so a rolling deploy does not strand rows in SENDING.
func Start(ctx context.Context) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		dispatch(ctx)
	}()
	go func() {
		defer wg.Done()
		reap(ctx)
	}()
	logger.SugarLogger.Infof("Delivery worker started with concurrency %d", config.WorkerConcurrencyLimit)
	return &wg
}

func dispatch(ctx context.Context) {
	slots := make(chan struct{}, config.WorkerConcurrencyLimit)
	var inFlight sync.WaitGroup
	defer inFlight.Wait()

	for {
		if ctx.Err() != nil {
			return
		}
		claimed, err := service.ClaimDeliveries(config.WorkerConcurrencyLimit * claimMultiplier)
		if err != nil {
			logger.SugarLogger.Errorf("failed to claim deliveries: %v", err)
		}
		messages := map[string]model.Message{}
		for i, delivery := range claimed {
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				release(claimed[i:])
				return
			}
			inFlight.Add(1)
			message, err := loadMessage(messages, delivery.MessageID)
			go func() {
				defer func() { <-slots; inFlight.Done() }()
				if err != nil {
					logger.SugarLogger.Errorf("failed to load message %s for delivery %s: %v", delivery.MessageID, delivery.ID, err)
					finish(delivery, retryOutcome(delivery, err, "", ""))
					return
				}
				process(delivery, message)
			}()
		}
		if len(claimed) == config.WorkerConcurrencyLimit*claimMultiplier {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

// release hands claimed-but-unstarted deliveries back on shutdown, without
// charging them an attempt, so they go out on the next replica immediately.
func release(deliveries []model.Delivery) {
	ids := make([]string, 0, len(deliveries))
	for _, delivery := range deliveries {
		ids = append(ids, delivery.ID)
	}
	if err := service.ReleaseClaimedDeliveries(ids); err != nil {
		logger.SugarLogger.Errorf("failed to release %d claimed deliveries on shutdown: %v", len(ids), err)
	}
}

func reap(ctx context.Context) {
	ticker := time.NewTicker(reapInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			released, err := service.ReleaseStaleDeliveries(staleLockAfter)
			if err != nil {
				logger.SugarLogger.Errorf("failed to release stale deliveries: %v", err)
			} else if released > 0 {
				logger.SugarLogger.Warnf("released %d stale deliveries back to the queue", released)
			}
		}
	}
}

func loadMessage(cache map[string]model.Message, id string) (model.Message, error) {
	if message, ok := cache[id]; ok {
		return message, nil
	}
	message, err := service.GetMessage(id)
	if err != nil {
		return model.Message{}, err
	}
	cache[id] = message
	return message, nil
}

// process runs on a context detached from shutdown: once a send starts it is
// allowed to finish, bounded by sendTimeout, rather than being cut off
// mid-request and left for the reaper.
func process(delivery model.Delivery, message model.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()

	sender, ok := senders[delivery.Channel]
	if !ok {
		finish(delivery, service.DeliveryOutcome{Status: model.DeliveryStatusFailed, LastError: "no sender for channel " + string(delivery.Channel)})
		return
	}

	contact, err := service.GetContact(ctx, delivery.EntityID)
	if err != nil {
		if errors.Is(err, service.ErrRecipientNotFound) {
			finish(delivery, service.DeliveryOutcome{Status: model.DeliveryStatusSkipped, Provider: sender.Provider(), LastError: err.Error()})
			return
		}
		finish(delivery, retryOutcome(delivery, err, sender.Provider(), ""))
		return
	}

	destination, ok := sender.Destination(contact)
	if !ok {
		finish(delivery, service.DeliveryOutcome{
			Status:    model.DeliveryStatusSkipped,
			Provider:  sender.Provider(),
			LastError: "recipient has no " + string(delivery.Channel) + " address on file",
		})
		return
	}
	masked := sender.MaskDestination(destination)

	providerMessageID, err := sender.Send(ctx, Outbound{
		DeliveryID: delivery.ID,
		Message:    message,
		Contact:    contact,
	}, destination)
	if err != nil {
		var permanent permanentError
		if errors.As(err, &permanent) && permanent.Permanent() {
			finish(delivery, service.DeliveryOutcome{Status: model.DeliveryStatusFailed, Provider: sender.Provider(), Destination: masked, LastError: err.Error()})
			return
		}
		finish(delivery, retryOutcome(delivery, err, sender.Provider(), masked))
		return
	}
	finish(delivery, service.DeliveryOutcome{
		Status:            model.DeliveryStatusSent,
		Provider:          sender.Provider(),
		Destination:       masked,
		ProviderMessageID: providerMessageID,
	})
}

func retryOutcome(delivery model.Delivery, err error, provider string, destination string) service.DeliveryOutcome {
	outcome := service.DeliveryOutcome{Provider: provider, Destination: destination, LastError: err.Error()}
	if delivery.Attempts >= config.DeliveryMaxAttemptsLimit {
		outcome.Status = model.DeliveryStatusFailed
		return outcome
	}
	outcome.Status = model.DeliveryStatusPending
	outcome.NextAttemptAt = time.Now().Add(backoff(delivery.Attempts, err))
	return outcome
}

// backoff doubles from baseBackoff per attempt, jittered across the upper
// half of the window so retries from one outage spread out, unless the
// provider said exactly when to come back.
func backoff(attempts int, err error) time.Duration {
	var retryAfter retryAfterError
	if errors.As(err, &retryAfter) {
		if wait := retryAfter.RetryAfterDuration(); wait > 0 {
			return min(wait, maxRetryAfter)
		}
	}
	ceiling := min(baseBackoff<<max(attempts-1, 0), maxBackoff)
	return ceiling/2 + rand.N(ceiling/2)
}

func finish(delivery model.Delivery, outcome service.DeliveryOutcome) {
	if err := service.CompleteDelivery(delivery.ID, outcome); err != nil {
		if errors.Is(err, service.ErrDeliveryReassigned) {
			logger.SugarLogger.Warnf("delivery %s was reclaimed before its result was recorded", delivery.ID)
			return
		}
		logger.SugarLogger.Errorf("failed to record outcome for delivery %s: %v", delivery.ID, err)
	}
}
