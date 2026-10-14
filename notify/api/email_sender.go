package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"

	"github.com/gaucho-racing/notify/notify/service"
	"github.com/gin-gonic/gin"
)

const (
	maxEmailSenderNameLength        = 64
	maxEmailSenderDescriptionLength = 500
	maxEmailFromNameLength          = 100
)

type emailSenderRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	FromName    *string `json:"from_name"`
	FromAddress *string `json:"from_address"`
	ReplyTo     *string `json:"reply_to"`
	IsDefault   *bool   `json:"is_default"`
	Enabled     *bool   `json:"enabled"`
}

// emailSenderSummary is what applications see: enough to pick a sender by
// ID, nothing about who configured it.
type emailSenderSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	From        string `json:"from"`
	ReplyTo     string `json:"reply_to"`
	IsDefault   bool   `json:"is_default"`
	Enabled     bool   `json:"enabled"`
}

// ListEmailSenders is open to applications as well as admins so a sending
// app can discover which email_sender_id to pass.
func ListEmailSenders(c *gin.Context) {
	Require(c, RequestTokenExists(c))
	isAdmin := RequestTokenIsAdmin(c)
	Require(c, isAdmin || !RequestTokenIsFirstParty(c))

	senders, err := service.ListEmailSenders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if isAdmin {
		c.JSON(http.StatusOK, senders)
		return
	}
	summaries := make([]emailSenderSummary, 0, len(senders))
	for _, sender := range senders {
		summaries = append(summaries, emailSenderSummary{
			ID:          sender.ID,
			Name:        sender.Name,
			Description: sender.Description,
			From:        sender.From(),
			ReplyTo:     sender.ReplyTo,
			IsDefault:   sender.IsDefault,
			Enabled:     sender.Enabled,
		})
	}
	c.JSON(http.StatusOK, summaries)
}

func GetEmailSender(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	sender, err := service.GetEmailSender(c.Param("senderID"))
	if err != nil {
		respondEmailSenderError(c, err)
		return
	}
	c.JSON(http.StatusOK, sender)
}

func CreateEmailSender(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	input, ok := bindEmailSenderRequest(c)
	if !ok {
		return
	}
	if input.Name == nil || input.FromAddress == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and from_address are required"})
		return
	}
	sender, err := service.CreateEmailSender(c.Request.Context(), input, requestActor(c))
	if err != nil {
		respondEmailSenderError(c, err)
		return
	}
	c.JSON(http.StatusCreated, sender)
}

func UpdateEmailSender(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	input, ok := bindEmailSenderRequest(c)
	if !ok {
		return
	}
	sender, err := service.UpdateEmailSender(c.Request.Context(), c.Param("senderID"), input, requestActor(c))
	if err != nil {
		respondEmailSenderError(c, err)
		return
	}
	c.JSON(http.StatusOK, sender)
}

func DeleteEmailSender(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	if err := service.DeleteEmailSender(c.Param("senderID")); err != nil {
		respondEmailSenderError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func bindEmailSenderRequest(c *gin.Context) (service.EmailSenderInput, bool) {
	var req emailSenderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return service.EmailSenderInput{}, false
	}
	input := service.EmailSenderInput{IsDefault: req.IsDefault, Enabled: req.Enabled}
	fail := func(message string) (service.EmailSenderInput, bool) {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return service.EmailSenderInput{}, false
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len([]rune(name)) > maxEmailSenderNameLength {
			return fail(fmt.Sprintf("name must be 1-%d characters", maxEmailSenderNameLength))
		}
		input.Name = &name
	}
	if req.Description != nil {
		description := strings.TrimSpace(*req.Description)
		if len([]rune(description)) > maxEmailSenderDescriptionLength {
			return fail(fmt.Sprintf("description must be at most %d characters", maxEmailSenderDescriptionLength))
		}
		input.Description = &description
	}
	if req.FromName != nil {
		fromName := strings.TrimSpace(*req.FromName)
		if len([]rune(fromName)) > maxEmailFromNameLength || strings.ContainsAny(fromName, "\r\n") {
			return fail(fmt.Sprintf("from_name must be at most %d characters on one line", maxEmailFromNameLength))
		}
		input.FromName = &fromName
	}
	if req.FromAddress != nil {
		address, ok := bareEmailAddress(*req.FromAddress)
		if !ok {
			return fail("from_address must be a plain email address, e.g. team@notify.gauchoracing.com")
		}
		input.FromAddress = &address
	}
	if req.ReplyTo != nil {
		replyTo := strings.TrimSpace(*req.ReplyTo)
		if replyTo != "" {
			address, ok := bareEmailAddress(replyTo)
			if !ok {
				return fail("reply_to must be a plain email address")
			}
			replyTo = address
		}
		input.ReplyTo = &replyTo
	}
	return input, true
}

// bareEmailAddress accepts only "local@domain"; display names belong in
// from_name so the stored address is always comparable to Resend domains.
func bareEmailAddress(value string) (string, bool) {
	value = strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Name != "" || parsed.Address != value || !strings.Contains(value, "@") {
		return "", false
	}
	local, domain, _ := strings.Cut(value, "@")
	return local + "@" + strings.ToLower(domain), true
}

func respondEmailSenderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrEmailSenderNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrEmailSenderNameTaken), errors.Is(err, service.ErrDefaultEmailSenderHeld):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrEmailDomainNotVerified):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrResendUnreachable):
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
