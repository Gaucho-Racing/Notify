package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/discord"
	"github.com/gaucho-racing/notify/notify/pkg/twilio"
	"github.com/gaucho-racing/notify/notify/service"
	"github.com/gin-gonic/gin"
)

const (
	maxSubjectLength  = 200
	maxBodyLength     = 10_000
	maxHTMLBytes      = 100_000
	maxCategoryLength = 64
	maxMetadataBytes  = 8_000
	defaultPageSize   = 50
	maxPageSize       = 200
)

type sendMessageRequest struct {
	Recipients model.MessageRecipients `json:"recipients"`
	Channels   []model.Channel         `json:"channels"`
	Subject    string                  `json:"subject"`
	Body       string                  `json:"body"`
	Category   string                  `json:"category"`
	Metadata   map[string]any          `json:"metadata"`
	Email      *emailOptions           `json:"email"`
	SMS        *smsOptions             `json:"sms"`
	Discord    *discordOptions         `json:"discord"`
}

// Each channel block picks that channel's sender and, for email and Discord,
// an optional rich version. Without one, every channel sends Body as plain
// text.
type emailOptions struct {
	SenderID string `json:"sender_id"`
	HTML     string `json:"html"`
}

type smsOptions struct {
	SenderID string `json:"sender_id"`
}

type discordOptions struct {
	BotID  string          `json:"bot_id"`
	Embeds []discord.Embed `json:"embeds"`
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
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
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
			errors.Is(err, service.ErrResendAccountNotFound), errors.Is(err, service.ErrResendAccountDisabled),
			errors.Is(err, service.ErrSMSSenderNotFound), errors.Is(err, service.ErrSMSSenderDisabled), errors.Is(err, service.ErrNoDefaultSMSSender),
			errors.Is(err, service.ErrTwilioAccountNotFound), errors.Is(err, service.ErrTwilioAccountDisabled):
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
	case len([]rune(subject)) > maxSubjectLength:
		return service.SendRequest{}, fmt.Errorf("subject must be at most %d characters", maxSubjectLength)
	case body == "":
		return service.SendRequest{}, errors.New("body is required")
	case len([]rune(body)) > maxBodyLength:
		return service.SendRequest{}, fmt.Errorf("body must be at most %d characters", maxBodyLength)
	case len([]rune(req.Category)) > maxCategoryLength:
		return service.SendRequest{}, fmt.Errorf("category must be at most %d characters", maxCategoryLength)
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
	sendReq := service.SendRequest{
		Recipients: req.Recipients,
		Channels:   channels,
		Subject:    subject,
		Body:       body,
		Category:   strings.TrimSpace(req.Category),
		Metadata:   req.Metadata,
	}
	if sendReq.Recipients.EntityIDs == nil {
		sendReq.Recipients.EntityIDs = []string{}
	}
	if sendReq.Recipients.GroupIDs == nil {
		sendReq.Recipients.GroupIDs = []string{}
	}
	requireChannel := func(block string, channel model.Channel) error {
		if !slices.Contains(channels, channel) {
			return fmt.Errorf("%s requires %s in channels", block, channel)
		}
		return nil
	}

	if slices.Contains(channels, model.ChannelEmail) && subject == "" {
		return service.SendRequest{}, errors.New("subject is required when sending EMAIL")
	}
	if req.Email != nil {
		if err := requireChannel("email", model.ChannelEmail); err != nil {
			return service.SendRequest{}, err
		}
		if len(req.Email.HTML) > maxHTMLBytes {
			return service.SendRequest{}, fmt.Errorf("email.html must be at most %d bytes", maxHTMLBytes)
		}
		sendReq.EmailSenderID = strings.TrimSpace(req.Email.SenderID)
		sendReq.EmailHTML = strings.TrimSpace(req.Email.HTML)
	}

	if slices.Contains(channels, model.ChannelSMS) {
		if length := twilio.BodyLength(body); length > twilio.MaxBodyLength {
			return service.SendRequest{}, fmt.Errorf("body is %d characters, over SMS's %d-character limit", length, twilio.MaxBodyLength)
		}
	}
	if req.SMS != nil {
		if err := requireChannel("sms", model.ChannelSMS); err != nil {
			return service.SendRequest{}, err
		}
		sendReq.SMSSenderID = strings.TrimSpace(req.SMS.SenderID)
	}

	if req.Discord != nil {
		if err := requireChannel("discord", model.ChannelDiscord); err != nil {
			return service.SendRequest{}, err
		}
		if err := discord.ValidateEmbeds(req.Discord.Embeds); err != nil {
			return service.SendRequest{}, fmt.Errorf("discord.%w", err)
		}
		sendReq.DiscordBotID = strings.TrimSpace(req.Discord.BotID)
		sendReq.DiscordEmbeds = req.Discord.Embeds
	}
	if slices.Contains(channels, model.ChannelDiscord) && len(sendReq.DiscordEmbeds) == 0 && utf8.RuneCountInString(body) > discord.MaxContentLength {
		return service.SendRequest{}, fmt.Errorf("body is over Discord's %d-character message limit; shorten it or send discord.embeds", discord.MaxContentLength)
	}

	if req.Metadata != nil {
		encoded, err := json.Marshal(req.Metadata)
		if err != nil || len(encoded) > maxMetadataBytes {
			return service.SendRequest{}, fmt.Errorf("metadata must encode to at most %d bytes of JSON", maxMetadataBytes)
		}
	}
	return sendReq, nil
}
