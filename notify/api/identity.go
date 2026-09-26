package api

import (
	"net/http"
	"strings"

	"github.com/gaucho-racing/notify/notify/service"
	"github.com/gin-gonic/gin"
)

const maxIdentityIDs = 100

const maxApplicationClientIDs = 100

type identityRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

type applicationRequest struct {
	ClientIDs []string `json:"client_ids" binding:"required"`
}

func ResolveIdentities(c *gin.Context) {
	Require(c, RequestTokenExists(c))

	var req identityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids is required"})
		return
	}
	if len(req.IDs) > maxIdentityIDs {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at most 100 entity IDs may be resolved at once"})
		return
	}
	for _, entityID := range req.IDs {
		if strings.TrimSpace(entityID) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "entity IDs must not be empty"})
			return
		}
	}

	summaries, err := service.ResolveIdentities(c.Request.Context(), req.IDs)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summaries)
}

func ResolveApplications(c *gin.Context) {
	Require(c, RequestTokenExists(c))

	var req applicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client_ids is required"})
		return
	}
	if len(req.ClientIDs) > maxApplicationClientIDs {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at most 100 application client IDs may be resolved at once"})
		return
	}
	for _, clientID := range req.ClientIDs {
		if strings.TrimSpace(clientID) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "application client IDs must not be empty"})
			return
		}
	}

	applications, err := service.ResolveApplications(req.ClientIDs)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, applications)
}
