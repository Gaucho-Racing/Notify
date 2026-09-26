package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/service"
	"github.com/gin-gonic/gin"
)

func GetStats(c *gin.Context) {
	filter, ok := statsFilter(c)
	if !ok {
		return
	}
	stats, err := service.GetStats(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func GetActivityStats(c *gin.Context) {
	filter, ok := statsFilter(c)
	if !ok {
		return
	}
	points, err := service.GetActivityStats(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, points)
}

func statsFilter(c *gin.Context) (service.StatsFilter, bool) {
	Require(c, RequestTokenExists(c))
	clientID, allowed := scopedClientID(c)
	Require(c, allowed)

	days, err := strconv.Atoi(c.DefaultQuery("days", "30"))
	if err != nil || days < 1 || days > 365 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "days must be an integer between 1 and 365"})
		return service.StatsFilter{}, false
	}
	filter := service.StatsFilter{ClientID: clientID, Days: days}
	if channel := model.Channel(strings.ToUpper(strings.TrimSpace(c.Query("channel")))); channel != "" {
		if !channel.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported channel: " + string(channel)})
			return service.StatsFilter{}, false
		}
		filter.Channel = channel
	}
	return filter, true
}
