package service

import (
	"errors"
	"net/url"
	"strings"

	"github.com/gaucho-racing/notify/notify/config"
	"github.com/gaucho-racing/notify/notify/database"
	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/twilio"
	"gorm.io/gorm"
)

const twilioReceiptPurpose = "twilio-status-callback"

// ErrReceiptBeforeSend means the receipt beat the worker recording Twilio's
// message SID, which can happen when a carrier rejects a message instantly.
var ErrReceiptBeforeSend = errors.New("delivery has not recorded its provider message yet")

var finalTwilioStatuses = []string{twilio.StatusDelivered, twilio.StatusUndelivered, twilio.StatusFailed, "read", "canceled"}

// TwilioStatusCallbackURL is signed per delivery because Twilio signs its
// requests with the account auth token, which Notify never holds when an
// account is added with an API key.
func TwilioStatusCallbackURL(deliveryID string) string {
	if config.PublicAPIURL == "" {
		return ""
	}
	signature, err := config.Secrets.Sign(twilioReceiptPurpose, deliveryID)
	if err != nil {
		return ""
	}
	return strings.TrimRight(config.PublicAPIURL, "/") + "/webhooks/twilio/status/" + url.PathEscape(deliveryID) +
		"?sig=" + url.QueryEscape(signature)
}

func VerifyTwilioStatusCallback(deliveryID string, signature string) bool {
	return config.Secrets.Verify(twilioReceiptPurpose, deliveryID, signature)
}

// RecordTwilioReceipt applies a status receipt to its delivery. Receipts for
// an older attempt (different SID) or arriving after a final status are
// ignored, since Twilio does not guarantee they arrive in order.
func RecordTwilioReceipt(deliveryID string, messageSID string, status string, errorCode string) error {
	status = strings.ToLower(strings.TrimSpace(status))
	updates := map[string]interface{}{"provider_status": status}
	query := database.DB.Model(&model.Delivery{}).
		Where("id = ? AND provider_message_id = ?", deliveryID, messageSID).
		Where("COALESCE(provider_status, '') NOT IN ?", finalTwilioStatuses)
	if twilio.IsFailureStatus(status) {
		query = query.Where("status = ?", model.DeliveryStatusSent)
		updates["status"] = model.DeliveryStatusFailed
		updates["last_error"] = twilio.DeliveryErrorMessage(status, errorCode)
	}
	result := query.Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}

	var delivery model.Delivery
	if err := database.DB.Select("status", "provider_message_id").Where("id = ?", deliveryID).First(&delivery).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDeliveryNotFound
		}
		return err
	}
	if delivery.Status == model.DeliveryStatusSending && delivery.ProviderMessageID == "" {
		return ErrReceiptBeforeSend
	}
	return nil
}
