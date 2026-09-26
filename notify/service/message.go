package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gaucho-racing/notify/notify/database"
	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
	ulid "github.com/gaucho-racing/ulid-go"
	"gorm.io/gorm"
)

const MaxRecipientsPerMessage = 1000

var (
	ErrNoRecipients       = errors.New("message resolves to no recipients")
	ErrTooManyRecipients  = fmt.Errorf("message resolves to more than %d recipients", MaxRecipientsPerMessage)
	ErrDailyLimitExceeded = errors.New("application daily delivery limit exceeded")
	ErrGroupNotFound      = errors.New("group not found in Sentinel")
	ErrMessageNotFound    = errors.New("message not found")
)

type SendRequest struct {
	Recipients  model.MessageRecipients
	Channels    []model.Channel
	Subject     string
	Body        string
	HTML        string
	ActionURL   string
	ActionLabel string
	Category    string
	Metadata    map[string]any
}

// SendMessage records a message and one PENDING delivery per recipient per
// channel; the worker does the actual sending. dailyLimit of 0 is unlimited.
func SendMessage(ctx context.Context, req SendRequest, actor Actor, dailyLimit int) (model.Message, error) {
	entityIDs, err := expandRecipients(ctx, req.Recipients)
	if err != nil {
		return model.Message{}, err
	}
	if len(entityIDs) == 0 {
		return model.Message{}, ErrNoRecipients
	}
	if len(entityIDs) > MaxRecipientsPerMessage {
		return model.Message{}, ErrTooManyRecipients
	}

	message := model.Message{
		ID:             ulid.Make().Prefixed("msg"),
		ClientID:       actor.ClientID,
		SenderEntityID: actor.EntityID,
		SenderType:     actor.Type,
		Subject:        req.Subject,
		Body:           req.Body,
		HTML:           req.HTML,
		ActionURL:      req.ActionURL,
		ActionLabel:    req.ActionLabel,
		Category:       req.Category,
		Metadata:       req.Metadata,
		Channels:       req.Channels,
		Recipients:     req.Recipients,
		RecipientCount: len(entityIDs),
	}
	if message.Metadata == nil {
		message.Metadata = map[string]any{}
	}

	now := time.Now()
	deliveries := make([]model.Delivery, 0, len(entityIDs)*len(req.Channels))
	for _, entityID := range entityIDs {
		for _, channel := range req.Channels {
			deliveries = append(deliveries, model.Delivery{
				ID:            ulid.Make().Prefixed("dlv"),
				MessageID:     message.ID,
				ClientID:      actor.ClientID,
				EntityID:      entityID,
				Channel:       channel,
				Status:        model.DeliveryStatusPending,
				NextAttemptAt: now,
			})
		}
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if dailyLimit > 0 {
			// Serializes concurrent sends from one application so two
			// requests cannot both pass the limit check before either commits.
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "notify_daily_limit:"+actor.ClientID).Error; err != nil {
				return err
			}
			var sentToday int64
			if err := tx.Model(&model.Delivery{}).
				Where("client_id = ? AND created_at > ?", actor.ClientID, now.Add(-24*time.Hour)).
				Count(&sentToday).Error; err != nil {
				return err
			}
			if int(sentToday)+len(deliveries) > dailyLimit {
				return ErrDailyLimitExceeded
			}
		}
		if err := tx.Create(&message).Error; err != nil {
			return err
		}
		return tx.CreateInBatches(&deliveries, 500).Error
	})
	if err != nil {
		return model.Message{}, err
	}
	return message, nil
}

func GetMessage(id string) (model.Message, error) {
	var message model.Message
	if err := database.DB.Where("id = ?", id).First(&message).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Message{}, ErrMessageNotFound
		}
		return model.Message{}, err
	}
	return message, nil
}

type MessageFilter struct {
	ClientID string
	Category string
	Before   string
	Limit    int
}

func ListMessages(filter MessageFilter) ([]model.Message, error) {
	query := database.DB.Model(&model.Message{})
	if filter.ClientID != "" {
		query = query.Where("client_id = ?", filter.ClientID)
	}
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if filter.Before != "" {
		query = query.Where("id < ?", filter.Before)
	}
	messages := []model.Message{}
	if err := query.Order("id desc").Limit(filter.Limit).Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

func expandRecipients(ctx context.Context, recipients model.MessageRecipients) ([]string, error) {
	seen := map[string]struct{}{}
	entityIDs := []string{}
	add := func(entityID string) {
		entityID = strings.TrimSpace(entityID)
		if entityID == "" {
			return
		}
		if _, exists := seen[entityID]; exists {
			return
		}
		seen[entityID] = struct{}{}
		entityIDs = append(entityIDs, entityID)
	}

	for _, entityID := range recipients.EntityIDs {
		add(entityID)
	}
	for _, groupID := range recipients.GroupIDs {
		members, err := sentinel.GetGroupMembers(ctx, groupID)
		if err != nil {
			var sentinelErr sentinel.Error
			if errors.As(err, &sentinelErr) && sentinelErr.Code == http.StatusNotFound {
				return nil, fmt.Errorf("%w: %s", ErrGroupNotFound, groupID)
			}
			return nil, fmt.Errorf("resolve group %s: %w", groupID, err)
		}
		for _, member := range members {
			add(member.EntityID)
		}
	}
	return entityIDs, nil
}
