package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/discord"
	"github.com/gaucho-racing/notify/notify/pkg/twilio"
	"github.com/gaucho-racing/notify/notify/service"
	"github.com/gin-gonic/gin"
)

var discordSnowflake = regexp.MustCompile(`^[0-9]{17,20}$`)

const (
	maxSubjectLength  = 200
	maxBodyLength     = 10_000
	maxHTMLBytes      = 100_000
	maxCategoryLength = 64
	maxMetadataBytes  = 8_000
	defaultPageSize   = 50
	maxPageSize       = 200
)

// sendMessageRequest sends on exactly one channel, and only that channel's
// content block may be set.
type sendMessageRequest struct {
	Channel    model.Channel           `json:"channel"`
	Recipients model.MessageRecipients `json:"recipients"`
	Category   string                  `json:"category"`
	Metadata   map[string]any          `json:"metadata"`
	Email      *emailContent           `json:"email"`
	SMS        *smsContent             `json:"sms"`
	Discord    *discordContent         `json:"discord"`
}

type emailContent struct {
	SenderID string `json:"sender_id"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`
	HTML     string `json:"html"`
}

type smsContent struct {
	SenderID string `json:"sender_id"`
	Body     string `json:"body"`
}

type discordContent struct {
	BotID   string          `json:"bot_id"`
	Content string          `json:"content"`
	Embeds  []discord.Embed `json:"embeds"`
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
		if !policy.AllowsChannel(sendReq.Channel) {
			c.JSON(http.StatusForbidden, gin.H{"error": "this application is not allowed to send on " + string(sendReq.Channel)})
			return
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
	channel := model.Channel(strings.ToUpper(strings.TrimSpace(string(req.Channel))))
	if !channel.Valid() {
		return service.SendRequest{}, errors.New("channel must be EMAIL, SMS, or DISCORD")
	}
	blocks := map[model.Channel]bool{model.ChannelEmail: req.Email != nil, model.ChannelSMS: req.SMS != nil, model.ChannelDiscord: req.Discord != nil}
	for blockChannel, present := range blocks {
		if present && blockChannel != channel {
			return service.SendRequest{}, fmt.Errorf("%s content is not allowed when channel is %s", strings.ToLower(string(blockChannel)), channel)
		}
	}
	if !blocks[channel] {
		return service.SendRequest{}, fmt.Errorf("%s content is required", strings.ToLower(string(channel)))
	}

	recipients := req.Recipients
	switch {
	case len(recipients.EntityIDs) == 0 && len(recipients.GroupIDs) == 0 && len(recipients.DiscordChannelIDs) == 0:
		return service.SendRequest{}, errors.New("recipients needs entity_ids, group_ids, or discord_channel_ids")
	case len(recipients.EntityIDs)+len(recipients.DiscordChannelIDs) > service.MaxRecipientsPerMessage:
		return service.SendRequest{}, fmt.Errorf("at most %d recipients may be sent to at once", service.MaxRecipientsPerMessage)
	case len(recipients.DiscordChannelIDs) > 0 && channel != model.ChannelDiscord:
		return service.SendRequest{}, errors.New("recipients.discord_channel_ids requires channel DISCORD")
	case len([]rune(req.Category)) > maxCategoryLength:
		return service.SendRequest{}, fmt.Errorf("category must be at most %d characters", maxCategoryLength)
	}
	for _, id := range recipients.DiscordChannelIDs {
		if !discordSnowflake.MatchString(strings.TrimSpace(id)) {
			return service.SendRequest{}, fmt.Errorf("discord channel ID %q is not a valid Discord ID", id)
		}
	}
	if recipients.EntityIDs == nil {
		recipients.EntityIDs = []string{}
	}
	if recipients.GroupIDs == nil {
		recipients.GroupIDs = []string{}
	}
	if req.Metadata != nil {
		encoded, err := json.Marshal(req.Metadata)
		if err != nil || len(encoded) > maxMetadataBytes {
			return service.SendRequest{}, fmt.Errorf("metadata must encode to at most %d bytes of JSON", maxMetadataBytes)
		}
	}

	sendReq := service.SendRequest{
		Channel:    channel,
		Recipients: recipients,
		Category:   strings.TrimSpace(req.Category),
		Metadata:   req.Metadata,
	}
	var err error
	switch channel {
	case model.ChannelEmail:
		err = applyEmailContent(&sendReq, *req.Email)
	case model.ChannelSMS:
		err = applySMSContent(&sendReq, *req.SMS)
	case model.ChannelDiscord:
		err = applyDiscordContent(&sendReq, *req.Discord)
	}
	return sendReq, err
}

func applyEmailContent(sendReq *service.SendRequest, email emailContent) error {
	subject := strings.TrimSpace(email.Subject)
	body := strings.TrimSpace(email.Body)
	switch {
	case subject == "":
		return errors.New("email.subject is required")
	case len([]rune(subject)) > maxSubjectLength:
		return fmt.Errorf("email.subject must be at most %d characters", maxSubjectLength)
	case body == "":
		return errors.New("email.body is required; it is the plain-text version even when html is set")
	case len([]rune(body)) > maxBodyLength:
		return fmt.Errorf("email.body must be at most %d characters", maxBodyLength)
	case len(email.HTML) > maxHTMLBytes:
		return fmt.Errorf("email.html must be at most %d bytes", maxHTMLBytes)
	}
	sendReq.SenderID = strings.TrimSpace(email.SenderID)
	sendReq.Subject = subject
	sendReq.Body = body
	sendReq.EmailHTML = strings.TrimSpace(email.HTML)
	return nil
}

func applySMSContent(sendReq *service.SendRequest, sms smsContent) error {
	body := strings.TrimSpace(sms.Body)
	if body == "" {
		return errors.New("sms.body is required")
	}
	if length := twilio.BodyLength(body); length > twilio.MaxBodyLength {
		return fmt.Errorf("sms.body is %d characters, over SMS's %d-character limit", length, twilio.MaxBodyLength)
	}
	sendReq.SenderID = strings.TrimSpace(sms.SenderID)
	sendReq.Body = body
	return nil
}

func applyDiscordContent(sendReq *service.SendRequest, content discordContent) error {
	text := strings.TrimSpace(content.Content)
	if text == "" && len(content.Embeds) == 0 {
		return errors.New("discord needs content, embeds, or both")
	}
	if utf8.RuneCountInString(text) > discord.MaxContentLength {
		return fmt.Errorf("discord.content must be at most %d characters", discord.MaxContentLength)
	}
	if err := discord.ValidateEmbeds(content.Embeds); err != nil {
		return fmt.Errorf("discord.%w", err)
	}
	sendReq.SenderID = strings.TrimSpace(content.BotID)
	sendReq.Body = text
	sendReq.DiscordEmbeds = content.Embeds
	return nil
}
