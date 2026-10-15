package service

import (
	"errors"
	"time"

	"github.com/gaucho-racing/notify/notify/database"
	"github.com/gaucho-racing/notify/notify/model"
	"gorm.io/gorm"
)

var (
	ErrDeliveryNotFound   = errors.New("delivery not found")
	ErrDeliveryNotFailed  = errors.New("only failed deliveries can be retried")
	ErrDeliveryReassigned = errors.New("delivery is no longer held by this worker")
)

type DeliveryFilter struct {
	ClientID  string
	MessageID string
	EntityID  string
	Channel   model.Channel
	Status    model.DeliveryStatus
	Before    string
	Limit     int
}

func ListDeliveries(filter DeliveryFilter) ([]model.Delivery, error) {
	query := database.DB.Model(&model.Delivery{})
	if filter.ClientID != "" {
		query = query.Where("client_id = ?", filter.ClientID)
	}
	if filter.MessageID != "" {
		query = query.Where("message_id = ?", filter.MessageID)
	}
	if filter.EntityID != "" {
		query = query.Where("entity_id = ?", filter.EntityID)
	}
	if filter.Channel != "" {
		query = query.Where("channel = ?", filter.Channel)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Before != "" {
		query = query.Where("id < ?", filter.Before)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	deliveries := []model.Delivery{}
	if err := query.Order("id desc").Find(&deliveries).Error; err != nil {
		return nil, err
	}
	return deliveries, nil
}

func GetDelivery(id string) (model.Delivery, error) {
	var delivery model.Delivery
	if err := database.DB.Where("id = ?", id).First(&delivery).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Delivery{}, ErrDeliveryNotFound
		}
		return model.Delivery{}, err
	}
	return delivery, nil
}

// RetryDelivery requeues a failed delivery with a fresh attempt budget.
func RetryDelivery(id string) (model.Delivery, error) {
	result := database.DB.Model(&model.Delivery{}).
		Where("id = ? AND status = ?", id, model.DeliveryStatusFailed).
		Updates(map[string]interface{}{
			"status":              model.DeliveryStatusPending,
			"attempts":            0,
			"next_attempt_at":     time.Now(),
			"last_error":          "",
			"provider_message_id": "",
			"provider_status":     "",
		})
	if result.Error != nil {
		return model.Delivery{}, result.Error
	}
	if result.RowsAffected == 0 {
		if _, err := GetDelivery(id); err != nil {
			return model.Delivery{}, err
		}
		return model.Delivery{}, ErrDeliveryNotFailed
	}
	return GetDelivery(id)
}

// ClaimDeliveries atomically moves up to limit due PENDING deliveries to
// SENDING and returns them. SKIP LOCKED lets any number of Notify replicas
// poll the same table without handing one delivery to two workers.
func ClaimDeliveries(limit int) ([]model.Delivery, error) {
	deliveries := []model.Delivery{}
	err := database.DB.Raw(`
		UPDATE notify_delivery
		SET status = ?, locked_at = now(), attempts = attempts + 1, updated_at = now()
		WHERE id IN (
			SELECT id FROM notify_delivery
			WHERE status = ? AND next_attempt_at <= now()
			ORDER BY next_attempt_at
			LIMIT ?
			FOR UPDATE SKIP LOCKED
		)
		RETURNING *`,
		model.DeliveryStatusSending, model.DeliveryStatusPending, limit,
	).Scan(&deliveries).Error
	return deliveries, err
}

// ReleaseStaleDeliveries returns SENDING rows whose worker died mid-send to
// the queue. A released email is safe to resend because the delivery ID is
// its Resend idempotency key; a Discord DM may be sent twice.
func ReleaseStaleDeliveries(olderThan time.Duration) (int64, error) {
	result := database.DB.Model(&model.Delivery{}).
		Where("status = ? AND locked_at < ?", model.DeliveryStatusSending, time.Now().Add(-olderThan)).
		Updates(map[string]interface{}{
			"status":          model.DeliveryStatusPending,
			"locked_at":       nil,
			"next_attempt_at": time.Now(),
		})
	return result.RowsAffected, result.Error
}

func ReleaseClaimedDeliveries(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return database.DB.Model(&model.Delivery{}).
		Where("id IN ? AND status = ?", ids, model.DeliveryStatusSending).
		Updates(map[string]interface{}{
			"status":    model.DeliveryStatusPending,
			"locked_at": nil,
			"attempts":  gorm.Expr("attempts - 1"),
		}).Error
}

type DeliveryOutcome struct {
	Status            model.DeliveryStatus
	Provider          string
	Destination       string
	ProviderMessageID string
	LastError         string
	NextAttemptAt     time.Time
}

// CompleteDelivery records the outcome of an attempt. The SENDING guard makes
// a worker that outlived the stale-lock window lose to whichever worker
// reclaimed the row, instead of overwriting its result.
func CompleteDelivery(id string, outcome DeliveryOutcome) error {
	updates := map[string]interface{}{
		"status":      outcome.Status,
		"locked_at":   nil,
		"last_error":  outcome.LastError,
		"provider":    outcome.Provider,
		"destination": outcome.Destination,
	}
	switch outcome.Status {
	case model.DeliveryStatusSent:
		updates["sent_at"] = time.Now()
		updates["provider_message_id"] = outcome.ProviderMessageID
	case model.DeliveryStatusPending:
		updates["next_attempt_at"] = outcome.NextAttemptAt
	}
	result := database.DB.Model(&model.Delivery{}).
		Where("id = ? AND status = ?", id, model.DeliveryStatusSending).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDeliveryReassigned
	}
	return nil
}
