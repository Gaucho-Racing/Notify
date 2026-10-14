package api

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gaucho-racing/notify/notify/pkg/secretbox"
	"github.com/gaucho-racing/notify/notify/service"
	"github.com/gin-gonic/gin"
)

const (
	maxTwilioAccountNameLength        = 64
	maxTwilioAccountDescriptionLength = 500
)

var (
	twilioAccountSIDPattern = regexp.MustCompile(`^AC[0-9a-fA-F]{32}$`)
	twilioKeySIDPattern     = regexp.MustCompile(`^SK[0-9a-fA-F]{32}$`)
)

type twilioAccountRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	AccountSID  *string `json:"account_sid"`
	KeySID      *string `json:"key_sid"`
	Secret      *string `json:"secret"`
	Enabled     *bool   `json:"enabled"`
}

func ListTwilioAccounts(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	accounts, err := service.ListTwilioAccounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, accounts)
}

func GetTwilioAccount(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	account, err := service.GetTwilioAccount(c.Param("accountID"))
	if err != nil {
		respondTwilioAccountError(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

func ListTwilioSenderOptions(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	options, err := service.ListTwilioSenderOptions(c.Request.Context(), c.Param("accountID"))
	if err != nil {
		respondTwilioAccountError(c, err)
		return
	}
	c.JSON(http.StatusOK, options)
}

func CreateTwilioAccount(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	input, ok := bindTwilioAccountRequest(c)
	if !ok {
		return
	}
	if input.Name == nil || input.AccountSID == nil || input.Secret == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, account_sid, and secret are required"})
		return
	}
	account, err := service.CreateTwilioAccount(c.Request.Context(), input, requestActor(c))
	if err != nil {
		respondTwilioAccountError(c, err)
		return
	}
	c.JSON(http.StatusCreated, account)
}

func UpdateTwilioAccount(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	input, ok := bindTwilioAccountRequest(c)
	if !ok {
		return
	}
	if input.AccountSID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_sid cannot be changed; add a new Twilio account instead"})
		return
	}
	if input.KeySID != nil && input.Secret == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "secret is required when changing key_sid"})
		return
	}
	account, err := service.UpdateTwilioAccount(c.Request.Context(), c.Param("accountID"), input, requestActor(c))
	if err != nil {
		respondTwilioAccountError(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

func DeleteTwilioAccount(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	if err := service.DeleteTwilioAccount(c.Param("accountID")); err != nil {
		respondTwilioAccountError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func bindTwilioAccountRequest(c *gin.Context) (service.TwilioAccountInput, bool) {
	var req twilioAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return service.TwilioAccountInput{}, false
	}
	input := service.TwilioAccountInput{Enabled: req.Enabled}
	fail := func(message string) (service.TwilioAccountInput, bool) {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return service.TwilioAccountInput{}, false
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len([]rune(name)) > maxTwilioAccountNameLength {
			return fail(fmt.Sprintf("name must be 1-%d characters", maxTwilioAccountNameLength))
		}
		input.Name = &name
	}
	if req.Description != nil {
		description := strings.TrimSpace(*req.Description)
		if len([]rune(description)) > maxTwilioAccountDescriptionLength {
			return fail(fmt.Sprintf("description must be at most %d characters", maxTwilioAccountDescriptionLength))
		}
		input.Description = &description
	}
	if req.AccountSID != nil {
		accountSID := strings.TrimSpace(*req.AccountSID)
		if !twilioAccountSIDPattern.MatchString(accountSID) {
			return fail("account_sid must look like AC followed by 32 hex characters")
		}
		input.AccountSID = &accountSID
	}
	if req.KeySID != nil {
		keySID := strings.TrimSpace(*req.KeySID)
		if keySID != "" && !twilioKeySIDPattern.MatchString(keySID) {
			return fail("key_sid must be empty or look like SK followed by 32 hex characters")
		}
		input.KeySID = &keySID
	}
	if req.Secret != nil {
		secret := strings.TrimSpace(*req.Secret)
		if secret == "" {
			return fail("secret must not be empty")
		}
		input.Secret = &secret
	}
	return input, true
}

func respondTwilioAccountError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTwilioAccountNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrTwilioAccountNameTaken), errors.Is(err, service.ErrTwilioAccountInUse):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidTwilioCredentials):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, secretbox.ErrNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrTwilioUnreachable):
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
