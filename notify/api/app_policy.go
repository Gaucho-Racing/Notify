package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/service"
	"github.com/gin-gonic/gin"
)

type appPolicyRequest struct {
	Description     string          `json:"description"`
	AllowedChannels []model.Channel `json:"allowed_channels" binding:"required"`
	CanTargetGroups bool            `json:"can_target_groups"`
	DailyLimit      int             `json:"daily_limit"`
	Enabled         *bool           `json:"enabled"`
}

func ListAppPolicies(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	policies, err := service.ListAppPolicies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policies)
}

func GetAppPolicy(c *gin.Context) {
	clientID := strings.TrimSpace(c.Param("clientID"))
	Require(c, RequestTokenCanAccessClient(c, clientID))

	policy, err := service.GetAppPolicy(clientID)
	if err != nil {
		if errors.Is(err, service.ErrAppPolicyNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

func UpsertAppPolicy(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	clientID := strings.TrimSpace(c.Param("clientID"))
	var req appPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "allowed_channels is required"})
		return
	}
	channels, err := normalizeChannels(req.AllowedChannels)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.DailyLimit < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "daily_limit must not be negative"})
		return
	}

	applications, err := service.ResolveApplications([]string{clientID})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if len(applications) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no Sentinel application has client_id " + clientID})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	policy, err := service.UpsertAppPolicy(model.AppPolicy{
		ClientID:        clientID,
		Description:     strings.TrimSpace(req.Description),
		AllowedChannels: channels,
		CanTargetGroups: req.CanTargetGroups,
		DailyLimit:      req.DailyLimit,
		Enabled:         enabled,
	}, requestActor(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

func DeleteAppPolicy(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	if err := service.DeleteAppPolicy(strings.TrimSpace(c.Param("clientID"))); err != nil {
		if errors.Is(err, service.ErrAppPolicyNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func normalizeChannels(channels []model.Channel) ([]model.Channel, error) {
	seen := map[model.Channel]struct{}{}
	normalized := make([]model.Channel, 0, len(channels))
	for _, channel := range channels {
		channel = model.Channel(strings.ToUpper(strings.TrimSpace(string(channel))))
		if !channel.Valid() {
			return nil, errors.New("unsupported channel: " + string(channel))
		}
		if _, exists := seen[channel]; exists {
			continue
		}
		seen[channel] = struct{}{}
		normalized = append(normalized, channel)
	}
	return normalized, nil
}
