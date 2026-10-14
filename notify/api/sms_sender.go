package api

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gaucho-racing/notify/notify/pkg/secretbox"
	"github.com/gaucho-racing/notify/notify/pkg/twilio"
	"github.com/gaucho-racing/notify/notify/service"
	"github.com/gin-gonic/gin"
)

const (
	maxSMSSenderNameLength        = 64
	maxSMSSenderDescriptionLength = 500
)

var twilioMessagingServiceSIDPattern = regexp.MustCompile(`^MG[0-9a-fA-F]{32}$`)

type smsSenderRequest struct {
	Name                *string `json:"name"`
	Description         *string `json:"description"`
	TwilioAccountID     *string `json:"twilio_account_id"`
	FromNumber          *string `json:"from_number"`
	MessagingServiceSID *string `json:"messaging_service_sid"`
	IsDefault           *bool   `json:"is_default"`
	Enabled             *bool   `json:"enabled"`
}

// smsSenderSummary is what applications see: enough to pick a sender by ID,
// nothing about who configured it.
type smsSenderSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	From        string `json:"from"`
	IsDefault   bool   `json:"is_default"`
	Enabled     bool   `json:"enabled"`
}

// ListSMSSenders is open to applications as well as admins so a sending app
// can discover which sms_sender_id to pass.
func ListSMSSenders(c *gin.Context) {
	Require(c, RequestTokenExists(c))
	isAdmin := RequestTokenIsAdmin(c)
	Require(c, isAdmin || !RequestTokenIsFirstParty(c))

	senders, err := service.ListSMSSenders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if isAdmin {
		c.JSON(http.StatusOK, senders)
		return
	}
	summaries := make([]smsSenderSummary, 0, len(senders))
	for _, sender := range senders {
		summaries = append(summaries, smsSenderSummary{
			ID:          sender.ID,
			Name:        sender.Name,
			Description: sender.Description,
			From:        sender.From(),
			IsDefault:   sender.IsDefault,
			Enabled:     sender.Enabled,
		})
	}
	c.JSON(http.StatusOK, summaries)
}

func GetSMSSender(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	sender, err := service.GetSMSSender(c.Param("senderID"))
	if err != nil {
		respondSMSSenderError(c, err)
		return
	}
	c.JSON(http.StatusOK, sender)
}

func CreateSMSSender(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	input, ok := bindSMSSenderRequest(c)
	if !ok {
		return
	}
	if input.Name == nil || input.TwilioAccountID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and twilio_account_id are required"})
		return
	}
	sender, err := service.CreateSMSSender(c.Request.Context(), input, requestActor(c))
	if err != nil {
		respondSMSSenderError(c, err)
		return
	}
	c.JSON(http.StatusCreated, sender)
}

func UpdateSMSSender(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	input, ok := bindSMSSenderRequest(c)
	if !ok {
		return
	}
	sender, err := service.UpdateSMSSender(c.Request.Context(), c.Param("senderID"), input, requestActor(c))
	if err != nil {
		respondSMSSenderError(c, err)
		return
	}
	c.JSON(http.StatusOK, sender)
}

func DeleteSMSSender(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	if err := service.DeleteSMSSender(c.Param("senderID")); err != nil {
		respondSMSSenderError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// bindSMSSenderRequest accepts an empty from_number or messaging_service_sid
// so a sender can switch from one to the other in a single update.
func bindSMSSenderRequest(c *gin.Context) (service.SMSSenderInput, bool) {
	var req smsSenderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return service.SMSSenderInput{}, false
	}
	input := service.SMSSenderInput{IsDefault: req.IsDefault, Enabled: req.Enabled}
	fail := func(message string) (service.SMSSenderInput, bool) {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return service.SMSSenderInput{}, false
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len([]rune(name)) > maxSMSSenderNameLength {
			return fail(fmt.Sprintf("name must be 1-%d characters", maxSMSSenderNameLength))
		}
		input.Name = &name
	}
	if req.Description != nil {
		description := strings.TrimSpace(*req.Description)
		if len([]rune(description)) > maxSMSSenderDescriptionLength {
			return fail(fmt.Sprintf("description must be at most %d characters", maxSMSSenderDescriptionLength))
		}
		input.Description = &description
	}
	if req.TwilioAccountID != nil {
		accountID := strings.TrimSpace(*req.TwilioAccountID)
		if accountID == "" {
			return fail("twilio_account_id must not be empty")
		}
		input.TwilioAccountID = &accountID
	}
	if req.FromNumber != nil {
		fromNumber := strings.TrimSpace(*req.FromNumber)
		if fromNumber != "" {
			normalized, ok := twilio.NormalizePhoneNumber(fromNumber)
			if !ok {
				return fail("from_number must be in E.164 format, e.g. +18055551234")
			}
			fromNumber = normalized
		}
		input.FromNumber = &fromNumber
	}
	if req.MessagingServiceSID != nil {
		serviceSID := strings.TrimSpace(*req.MessagingServiceSID)
		if serviceSID != "" && !twilioMessagingServiceSIDPattern.MatchString(serviceSID) {
			return fail("messaging_service_sid must look like MG followed by 32 hex characters")
		}
		input.MessagingServiceSID = &serviceSID
	}
	return input, true
}

func respondSMSSenderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrSMSSenderNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrSMSSenderNameTaken), errors.Is(err, service.ErrDefaultSMSSenderHeld):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrSMSSenderSourceNeeded), errors.Is(err, service.ErrSMSSourceNotOnAccount),
		errors.Is(err, service.ErrTwilioAccountNotFound), errors.Is(err, service.ErrInvalidTwilioCredentials):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, secretbox.ErrNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrTwilioUnreachable):
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
