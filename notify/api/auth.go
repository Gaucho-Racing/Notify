package api

import (
	"fmt"
	"net/http"

	"github.com/gaucho-racing/notify/notify/config"
	"github.com/gaucho-racing/notify/notify/pkg/logger"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
	"github.com/gaucho-racing/notify/notify/service"
	"github.com/gin-gonic/gin"
)

func LoginWithSentinel(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	token, err := sentinel.ExchangeAuthorizationCode(code, callbackURL(c))
	if err != nil {
		logger.SugarLogger.Errorln("Failed to exchange Sentinel authorization code: " + err.Error())
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, token)
}

func GetSession(c *gin.Context) {
	Require(c, RequestTokenExists(c))
	// is_admin is resolved here so the web app never has to know which group
	// name Notify is configured to trust.
	c.JSON(http.StatusOK, gin.H{
		"entity_id":   GetRequestTokenEntityID(c),
		"user_id":     GetRequestTokenUserID(c),
		"client_id":   GetRequestTokenClientID(c),
		"scope":       GetRequestTokenScopes(c),
		"groups":      GetRequestTokenGroupNames(c),
		"first_party": RequestTokenIsFirstParty(c),
		"is_admin":    RequestTokenIsAdmin(c),
	})
}

func RefreshSession(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}
	token, err := sentinel.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, token)
}

func Logout(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func GetCurrentUser(c *gin.Context) {
	Require(c, RequestTokenExists(c))

	userID := GetRequestTokenUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token does not belong to a user"})
		return
	}
	user, err := sentinel.GetCurrentUser(GetRequestToken(c), userID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

func ListSentinelGroups(c *gin.Context) {
	Require(c, RequestTokenExists(c))

	groups, err := sentinel.GetGroups(GetRequestToken(c))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

func ListSentinelApplications(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	applications, err := service.ListApplications()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, applications)
}

func callbackURL(c *gin.Context) string {
	if config.SentinelRedirectURI != "" {
		return config.SentinelRedirectURI
	}
	return requestBaseURL(c) + "/auth/login"
}

func requestBaseURL(c *gin.Context) string {
	proto := c.GetHeader("X-Forwarded-Proto")
	if proto == "" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	return fmt.Sprintf("%s://%s", proto, host)
}
