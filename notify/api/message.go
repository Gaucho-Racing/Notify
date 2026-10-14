package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/service"
	"github.com/gin-gonic/gin"
)

const (
	maxSubjectLength     = 200
	maxBodyLength        = 10_000
	maxHTMLBytes         = 100_000
	maxCategoryLength    = 64
	maxActionLabelLength = 40
	maxMetadataBytes     = 8_000
	defaultPageSize      = 50
	maxPageSize          = 200
)

type sendMessageRequest struct {
	Recipients    model.MessageRecipients `json:"recipients"`
	Channels      []model.Channel         `json:"channels"`
	DiscordBotID  string                  `json:"discord_bot_id"`
	EmailSenderID string                  `json:"email_sender_id"`
	Subject       string                  `json:"subject"`
	Body          string                  `json:"body"`
	HTML          string                  `json:"html"`
	ActionURL     string                  `json:"action_url"`
	ActionLabel   string                  `json:"action_label"`
	Category      string                  `json:"category"`
	Metadata      map[string]any          `json:"metadata"`
}

type messageDetail struct {
	Message    model.Message                `json:"message"`
	Summary    map[model.DeliveryStatus]int `json:"summary"`
	Deliveries []model.Delivery             `json:"deliveries"`
}

// SendMessage is open to application service accounts under their policy and
// to Notify admins. User tokens minted for other applications are refused:
// any signed-in user of an app could otherwise message the whole team with
// that app's name on it.
func SendMessage(c *gin.Context) {
	Require(c, RequestTokenExists(c))
	isAdmin := RequestTokenIsAdmin(c)
	Require(c, isAdmin || (!RequestTokenIsFirstParty(c) && GetRequestActorType(c) == model.ActorTypeServiceAccount))

	var req sendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	sendReq, err := validateSendRequest(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dailyLimit := 0
	if !isAdmin {
		policy, err := service.GetAppPolicy(GetRequestTokenClientID(c))
		if err != nil {
			if errors.Is(err, service.ErrAppPolicyNotFound) {
				c.JSON(http.StatusForbidden, gin.H{"error": "this application has no Notify policy; ask a Notify admin to create one"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !policy.Enabled {
			c.JSON(http.StatusForbidden, gin.H{"error": "this application's Notify policy is disabled"})
			return
		}
		for _, channel := range sendReq.Channels {
			if !policy.AllowsChannel(channel) {
				c.JSON(http.StatusForbidden, gin.H{"error": "this application is not allowed to send on " + string(channel)})
				return
			}
		}
		if len(sendReq.Recipients.GroupIDs) > 0 && !policy.CanTargetGroups {
			c.JSON(http.StatusForbidden, gin.H{"error": "this application is not allowed to send to groups"})
			return
		}
		dailyLimit = policy.DailyLimit
	}

	message, err := service.SendMessage(c.Request.Context(), sendReq, requestActor(c), dailyLimit)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNoRecipients), errors.Is(err, service.ErrTooManyRecipients), errors.Is(err, service.ErrGroupNotFound),
			errors.Is(err, service.ErrDiscordBotNotFound), errors.Is(err, service.ErrDiscordBotDisabled), errors.Is(err, service.ErrNoDefaultDiscordBot),
			errors.Is(err, service.ErrEmailSenderNotFound), errors.Is(err, service.ErrEmailSenderDisabled), errors.Is(err, service.ErrNoDefaultEmailSender),
			errors.Is(err, service.ErrResendAccountNotFound), errors.Is(err, service.ErrResendAccountDisabled):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrDailyLimitExceeded):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusAccepted, message)
}

func ListMessages(c *gin.Context) {
	Require(c, RequestTokenExists(c))

	clientID, ok := scopedClientID(c)
	Require(c, ok)
	limit, err := pageSize(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	messages, err := service.ListMessages(service.MessageFilter{
		ClientID: clientID,
		Category: strings.TrimSpace(c.Query("category")),
		Before:   strings.TrimSpace(c.Query("before")),
		Limit:    limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, messages)
}

func GetMessage(c *gin.Context) {
	Require(c, RequestTokenExists(c))

	message, err := service.GetMessage(c.Param("messageID"))
	if err != nil {
		if errors.Is(err, service.ErrMessageNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	Require(c, RequestTokenCanAccessClient(c, message.ClientID))

	deliveries, err := service.ListDeliveries(service.DeliveryFilter{MessageID: message.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	summary := map[model.DeliveryStatus]int{}
	for _, delivery := range deliveries {
		summary[delivery.Status]++
	}
	c.JSON(http.StatusOK, messageDetail{Message: message, Summary: summary, Deliveries: deliveries})
}

func ListDeliveries(c *gin.Context) {
	Require(c, RequestTokenExists(c))

	clientID, ok := scopedClientID(c)
	Require(c, ok)
	limit, err := pageSize(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	filter := service.DeliveryFilter{
		ClientID:  clientID,
		MessageID: strings.TrimSpace(c.Query("message_id")),
		EntityID:  strings.TrimSpace(c.Query("entity_id")),
		Before:    strings.TrimSpace(c.Query("before")),
		Limit:     limit,
	}
	if channel := model.Channel(strings.ToUpper(strings.TrimSpace(c.Query("channel")))); channel != "" {
		if !channel.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported channel: " + string(channel)})
			return
		}
		filter.Channel = channel
	}
	if status := model.DeliveryStatus(strings.ToUpper(strings.TrimSpace(c.Query("status")))); status != "" {
		if !status.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported status: " + string(status)})
			return
		}
		filter.Status = status
	}

	deliveries, err := service.ListDeliveries(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, deliveries)
}

func RetryDelivery(c *gin.Context) {
	Require(c, RequestTokenExists(c))

	delivery, err := service.GetDelivery(c.Param("deliveryID"))
	if err != nil {
		if errors.Is(err, service.ErrDeliveryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	Require(c, RequestTokenCanAccessClient(c, delivery.ClientID))

	delivery, err = service.RetryDelivery(delivery.ID)
	if err != nil {
		if errors.Is(err, service.ErrDeliveryNotFailed) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, delivery)
}

// scopedClientID pins an application to its own client_id and lets admins
// filter by any (or none). ok is false for callers that may list nothing,
// i.e. non-admin sessions on Notify's own web client.
func scopedClientID(c *gin.Context) (string, bool) {
	if RequestTokenIsAdmin(c) {
		return strings.TrimSpace(c.Query("client_id")), true
	}
	if RequestTokenIsFirstParty(c) {
		return "", false
	}
	return GetRequestTokenClientID(c), GetRequestTokenClientID(c) != ""
}

func pageSize(c *gin.Context) (int, error) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultPageSize)))
	if err != nil || limit < 1 || limit > maxPageSize {
		return 0, fmt.Errorf("limit must be an integer between 1 and %d", maxPageSize)
	}
	return limit, nil
}

func validateSendRequest(req sendMessageRequest) (service.SendRequest, error) {
	subject := strings.TrimSpace(req.Subject)
	body := strings.TrimSpace(req.Body)
	switch {
	case subject == "":
		return service.SendRequest{}, errors.New("subject is required")
	case len([]rune(subject)) > maxSubjectLength:
		return service.SendRequest{}, fmt.Errorf("subject must be at most %d characters", maxSubjectLength)
	case body == "":
		return service.SendRequest{}, errors.New("body is required")
	case len([]rune(body)) > maxBodyLength:
		return service.SendRequest{}, fmt.Errorf("body must be at most %d characters", maxBodyLength)
	case len(req.HTML) > maxHTMLBytes:
		return service.SendRequest{}, fmt.Errorf("html must be at most %d bytes", maxHTMLBytes)
	case len([]rune(req.Category)) > maxCategoryLength:
		return service.SendRequest{}, fmt.Errorf("category must be at most %d characters", maxCategoryLength)
	case len([]rune(req.ActionLabel)) > maxActionLabelLength:
		return service.SendRequest{}, fmt.Errorf("action_label must be at most %d characters", maxActionLabelLength)
	case len(req.Recipients.EntityIDs) == 0 && len(req.Recipients.GroupIDs) == 0:
		return service.SendRequest{}, errors.New("recipients.entity_ids or recipients.group_ids is required")
	case len(req.Recipients.EntityIDs) > service.MaxRecipientsPerMessage:
		return service.SendRequest{}, fmt.Errorf("at most %d entity IDs may be sent to at once", service.MaxRecipientsPerMessage)
	case len(req.Channels) == 0:
		return service.SendRequest{}, errors.New("channels is required")
	}

	channels, err := normalizeChannels(req.Channels)
	if err != nil {
		return service.SendRequest{}, err
	}
	discordBotID := strings.TrimSpace(req.DiscordBotID)
	if discordBotID != "" && !slices.Contains(channels, model.ChannelDiscord) {
		return service.SendRequest{}, errors.New("discord_bot_id requires DISCORD in channels")
	}
	emailSenderID := strings.TrimSpace(req.EmailSenderID)
	if emailSenderID != "" && !slices.Contains(channels, model.ChannelEmail) {
		return service.SendRequest{}, errors.New("email_sender_id requires EMAIL in channels")
	}
	if req.Recipients.EntityIDs == nil {
		req.Recipients.EntityIDs = []string{}
	}
	if req.Recipients.GroupIDs == nil {
		req.Recipients.GroupIDs = []string{}
	}
	actionURL := strings.TrimSpace(req.ActionURL)
	if actionURL != "" {
		parsed, err := url.Parse(actionURL)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
			return service.SendRequest{}, errors.New("action_url must be an absolute http(s) URL")
		}
	}
	if req.Metadata != nil {
		encoded, err := json.Marshal(req.Metadata)
		if err != nil || len(encoded) > maxMetadataBytes {
			return service.SendRequest{}, fmt.Errorf("metadata must encode to at most %d bytes of JSON", maxMetadataBytes)
		}
	}

	return service.SendRequest{
		Recipients:    req.Recipients,
		Channels:      channels,
		DiscordBotID:  discordBotID,
		EmailSenderID: emailSenderID,
		Subject:       subject,
		Body:          body,
		HTML:          req.HTML,
		ActionURL:     actionURL,
		ActionLabel:   strings.TrimSpace(req.ActionLabel),
		Category:      strings.TrimSpace(req.Category),
		Metadata:      req.Metadata,
	}, nil
}
