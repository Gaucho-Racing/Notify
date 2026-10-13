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
	maxDiscordBotNameLength        = 64
	maxDiscordBotDescriptionLength = 500
)

type discordBotRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Token       *string `json:"token"`
	IsDefault   *bool   `json:"is_default"`
	Enabled     *bool   `json:"enabled"`
}

// discordBotSummary is what applications see: enough to pick a bot by ID,
// nothing about who configured it.
type discordBotSummary struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	DiscordUsername string `json:"discord_username"`
	AvatarURL       string `json:"avatar_url"`
	IsDefault       bool   `json:"is_default"`
	Enabled         bool   `json:"enabled"`
}

// ListDiscordBots is open to applications as well as admins so a sending app
// can discover which discord_bot_id to pass.
func ListDiscordBots(c *gin.Context) {
	Require(c, RequestTokenExists(c))
	isAdmin := RequestTokenIsAdmin(c)
	Require(c, isAdmin || !RequestTokenIsFirstParty(c))

	bots, err := service.ListDiscordBots()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if isAdmin {
		c.JSON(http.StatusOK, bots)
		return
	}
	summaries := make([]discordBotSummary, 0, len(bots))
	for _, bot := range bots {
		summaries = append(summaries, discordBotSummary{
			ID:              bot.ID,
			Name:            bot.Name,
			Description:     bot.Description,
			DiscordUsername: bot.DiscordUsername,
			AvatarURL:       bot.AvatarURL,
			IsDefault:       bot.IsDefault,
			Enabled:         bot.Enabled,
		})
	}
	c.JSON(http.StatusOK, summaries)
}

func GetDiscordBot(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	bot, err := service.GetDiscordBot(c.Param("botID"))
	if err != nil {
		respondDiscordBotError(c, err)
		return
	}
	c.JSON(http.StatusOK, bot)
}

func CreateDiscordBot(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	input, ok := bindDiscordBotRequest(c)
	if !ok {
		return
	}
	if input.Name == nil || input.Token == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and token are required"})
		return
	}
	bot, err := service.CreateDiscordBot(c.Request.Context(), input, requestActor(c))
	if err != nil {
		respondDiscordBotError(c, err)
		return
	}
	c.JSON(http.StatusCreated, bot)
}

func UpdateDiscordBot(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	input, ok := bindDiscordBotRequest(c)
	if !ok {
		return
	}
	bot, err := service.UpdateDiscordBot(c.Request.Context(), c.Param("botID"), input, requestActor(c))
	if err != nil {
		respondDiscordBotError(c, err)
		return
	}
	c.JSON(http.StatusOK, bot)
}

func DeleteDiscordBot(c *gin.Context) {
	Require(c, RequestTokenIsAdmin(c))

	if err := service.DeleteDiscordBot(c.Param("botID")); err != nil {
		respondDiscordBotError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func bindDiscordBotRequest(c *gin.Context) (service.DiscordBotInput, bool) {
	var req discordBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return service.DiscordBotInput{}, false
	}
	input := service.DiscordBotInput{IsDefault: req.IsDefault, Enabled: req.Enabled}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len([]rune(name)) > maxDiscordBotNameLength {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("name must be 1-%d characters", maxDiscordBotNameLength)})
			return service.DiscordBotInput{}, false
		}
		input.Name = &name
	}
	if req.Description != nil {
		description := strings.TrimSpace(*req.Description)
		if len([]rune(description)) > maxDiscordBotDescriptionLength {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("description must be at most %d characters", maxDiscordBotDescriptionLength)})
			return service.DiscordBotInput{}, false
		}
		input.Description = &description
	}
	if req.Token != nil {
		token := strings.TrimPrefix(strings.TrimSpace(*req.Token), "Bot ")
		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "token must not be empty"})
			return service.DiscordBotInput{}, false
		}
		input.Token = &token
	}
	return input, true
}

func respondDiscordBotError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDiscordBotNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrDiscordBotNameTaken), errors.Is(err, service.ErrDefaultDiscordBotHeld):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidDiscordToken), errors.Is(err, service.ErrDiscordTokenNotBot):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, secretbox.ErrNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrDiscordUnreachable):
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
