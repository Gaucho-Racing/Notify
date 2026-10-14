package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gaucho-racing/notify/notify/pkg/secretbox"
	"github.com/gaucho-racing/notify/notify/service"
	"github.com/gin-gonic/gin"
)

const (
	maxResendAccountNameLength        = 64
	maxResendAccountDescriptionLength = 500
)

type resendAccountRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	APIKey      *string `json:"api_key"`
	Enabled     *bool   `json:"enabled"`
}

func ListResendAccounts(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	accounts, err := service.ListResendAccounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, accounts)
}

func GetResendAccount(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	account, err := service.GetResendAccount(c.Param("accountID"))
	if err != nil {
		respondResendAccountError(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

func ListResendAccountDomains(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	domains, err := service.ListResendAccountDomains(c.Request.Context(), c.Param("accountID"))
	if err != nil {
		respondResendAccountError(c, err)
		return
	}
	c.JSON(http.StatusOK, domains)
}

func CreateResendAccount(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	input, ok := bindResendAccountRequest(c)
	if !ok {
		return
	}
	if input.Name == nil || input.APIKey == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and api_key are required"})
		return
	}
	account, err := service.CreateResendAccount(c.Request.Context(), input, requestActor(c))
	if err != nil {
		respondResendAccountError(c, err)
		return
	}
	c.JSON(http.StatusCreated, account)
}

func UpdateResendAccount(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	input, ok := bindResendAccountRequest(c)
	if !ok {
		return
	}
	account, err := service.UpdateResendAccount(c.Request.Context(), c.Param("accountID"), input, requestActor(c))
	if err != nil {
		respondResendAccountError(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

func DeleteResendAccount(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	if err := service.DeleteResendAccount(c.Param("accountID")); err != nil {
		respondResendAccountError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func bindResendAccountRequest(c *gin.Context) (service.ResendAccountInput, bool) {
	var req resendAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return service.ResendAccountInput{}, false
	}
	input := service.ResendAccountInput{Enabled: req.Enabled}
	fail := func(message string) (service.ResendAccountInput, bool) {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return service.ResendAccountInput{}, false
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len([]rune(name)) > maxResendAccountNameLength {
			return fail(fmt.Sprintf("name must be 1-%d characters", maxResendAccountNameLength))
		}
		input.Name = &name
	}
	if req.Description != nil {
		description := strings.TrimSpace(*req.Description)
		if len([]rune(description)) > maxResendAccountDescriptionLength {
			return fail(fmt.Sprintf("description must be at most %d characters", maxResendAccountDescriptionLength))
		}
		input.Description = &description
	}
	if req.APIKey != nil {
		apiKey := strings.TrimSpace(*req.APIKey)
		if apiKey == "" {
			return fail("api_key must not be empty")
		}
		input.APIKey = &apiKey
	}
	return input, true
}

func respondResendAccountError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrResendAccountNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrResendAccountNameTaken), errors.Is(err, service.ErrResendAccountInUse):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidResendKey), errors.Is(err, service.ErrResendKeySendingOnly):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, secretbox.ErrNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrResendUnreachable):
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
