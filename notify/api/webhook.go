package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gaucho-racing/notify/notify/pkg/logger"
	"github.com/gaucho-racing/notify/notify/service"
	"github.com/gin-gonic/gin"
)

const (
	receiptRetries    = 10
	receiptRetryDelay = 500 * time.Millisecond
)

func TwilioStatusCallback(c *gin.Context) {
	deliveryID := c.Param("deliveryID")
	if !service.VerifyTwilioStatusCallback(deliveryID, c.Query("sig")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid callback signature"})
		return
	}
	messageSID := c.PostForm("MessageSid")
	status := c.PostForm("MessageStatus")
	if messageSID == "" || status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MessageSid and MessageStatus are required"})
		return
	}

	for attempt := 0; ; attempt++ {
		err := service.RecordTwilioReceipt(deliveryID, messageSID, status, c.PostForm("ErrorCode"))
		if errors.Is(err, service.ErrReceiptBeforeSend) && attempt < receiptRetries {
			select {
			case <-time.After(receiptRetryDelay):
				continue
			case <-c.Request.Context().Done():
				return
			}
		}
		switch {
		case err == nil:
			c.Status(http.StatusNoContent)
		case errors.Is(err, service.ErrDeliveryNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrReceiptBeforeSend):
			logger.SugarLogger.Warnf("dropped twilio %s receipt for delivery %s: worker never recorded the message", status, deliveryID)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		default:
			logger.SugarLogger.Errorf("failed to record twilio receipt for delivery %s: %v", deliveryID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record receipt"})
		}
		return
	}
}
