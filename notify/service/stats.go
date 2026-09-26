package service

import (
	"fmt"
	"time"

	"github.com/gaucho-racing/notify/notify/database"
	"github.com/gaucho-racing/notify/notify/model"
	"gorm.io/gorm"
)

type StatusCounts struct {
	Pending int64 `json:"pending"`
	Sent    int64 `json:"sent"`
	Failed  int64 `json:"failed"`
	Skipped int64 `json:"skipped"`
}

func (s *StatusCounts) add(status model.DeliveryStatus, count int64) {
	switch status {
	case model.DeliveryStatusPending, model.DeliveryStatusSending:
		s.Pending += count
	case model.DeliveryStatusSent:
		s.Sent += count
	case model.DeliveryStatusFailed:
		s.Failed += count
	case model.DeliveryStatusSkipped:
		s.Skipped += count
	}
}

type ApplicationStats struct {
	ClientID   string       `json:"client_id"`
	Messages   int64        `json:"messages"`
	Deliveries StatusCounts `json:"deliveries"`
}

type ErrorCount struct {
	Channel model.Channel `json:"channel"`
	Error   string        `json:"error"`
	Count   int64         `json:"count"`
}

type Stats struct {
	Since        time.Time                      `json:"since"`
	Messages     int64                          `json:"messages"`
	Deliveries   StatusCounts                   `json:"deliveries"`
	ByChannel    map[model.Channel]StatusCounts `json:"by_channel"`
	Applications []ApplicationStats             `json:"applications"`
	TopErrors    []ErrorCount                   `json:"top_errors"`
}

type ActivityPoint struct {
	Date string `json:"date"`
	StatusCounts
}

// StatsFilter with an empty ClientID covers every application; callers are
// responsible for only passing that on behalf of admins.
type StatsFilter struct {
	ClientID string
	Channel  model.Channel
	Days     int
}

func (f StatsFilter) since() time.Time {
	return time.Now().UTC().AddDate(0, 0, -f.Days).Truncate(24 * time.Hour)
}

func (f StatsFilter) deliveries() *gorm.DB {
	query := database.DB.Model(&model.Delivery{}).Where("created_at >= ?", f.since())
	if f.ClientID != "" {
		query = query.Where("client_id = ?", f.ClientID)
	}
	if f.Channel != "" {
		query = query.Where("channel = ?", f.Channel)
	}
	return query
}

func (f StatsFilter) messages() *gorm.DB {
	query := database.DB.Model(&model.Message{}).Where("created_at >= ?", f.since())
	if f.ClientID != "" {
		query = query.Where("client_id = ?", f.ClientID)
	}
	return query
}

func GetStats(filter StatsFilter) (Stats, error) {
	stats := Stats{
		Since:        filter.since(),
		ByChannel:    map[model.Channel]StatusCounts{},
		Applications: []ApplicationStats{},
		TopErrors:    []ErrorCount{},
	}

	if err := filter.messages().Count(&stats.Messages).Error; err != nil {
		return Stats{}, fmt.Errorf("failed to count messages: %w", err)
	}

	rows := []struct {
		ClientID string
		Channel  model.Channel
		Status   model.DeliveryStatus
		Count    int64
	}{}
	if err := filter.deliveries().
		Select("client_id, channel, status, count(*) as count").
		Group("client_id, channel, status").
		Scan(&rows).Error; err != nil {
		return Stats{}, fmt.Errorf("failed to compute delivery stats: %w", err)
	}
	applications := map[string]*ApplicationStats{}
	for _, row := range rows {
		stats.Deliveries.add(row.Status, row.Count)
		channel := stats.ByChannel[row.Channel]
		channel.add(row.Status, row.Count)
		stats.ByChannel[row.Channel] = channel
		if _, exists := applications[row.ClientID]; !exists {
			applications[row.ClientID] = &ApplicationStats{ClientID: row.ClientID}
		}
		applications[row.ClientID].Deliveries.add(row.Status, row.Count)
	}

	messageCounts := []struct {
		ClientID string
		Count    int64
	}{}
	if err := filter.messages().
		Select("client_id, count(*) as count").
		Group("client_id").
		Scan(&messageCounts).Error; err != nil {
		return Stats{}, fmt.Errorf("failed to count messages per application: %w", err)
	}
	for _, row := range messageCounts {
		if _, exists := applications[row.ClientID]; !exists {
			applications[row.ClientID] = &ApplicationStats{ClientID: row.ClientID}
		}
		applications[row.ClientID].Messages = row.Count
	}
	for _, application := range applications {
		stats.Applications = append(stats.Applications, *application)
	}

	if err := filter.deliveries().
		Where("status = ?", model.DeliveryStatusFailed).
		Select("channel, last_error as error, count(*) as count").
		Group("channel, last_error").
		Order("count desc").
		Limit(10).
		Scan(&stats.TopErrors).Error; err != nil {
		return Stats{}, fmt.Errorf("failed to compute top errors: %w", err)
	}
	return stats, nil
}

func GetActivityStats(filter StatsFilter) ([]ActivityPoint, error) {
	since := filter.since()
	rows := []struct {
		Day    time.Time
		Status model.DeliveryStatus
		Count  int64
	}{}
	if err := filter.deliveries().
		Select("date_trunc('day', created_at) as day, status, count(*) as count").
		Group("day, status").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to compute activity stats: %w", err)
	}

	points := []ActivityPoint{}
	indexByDay := map[string]int{}
	for date := since; !date.After(time.Now().UTC()); date = date.AddDate(0, 0, 1) {
		key := date.Format("2006-01-02")
		indexByDay[key] = len(points)
		points = append(points, ActivityPoint{Date: key})
	}
	for _, row := range rows {
		if i, ok := indexByDay[row.Day.UTC().Format("2006-01-02")]; ok {
			points[i].add(row.Status, row.Count)
		}
	}
	return points, nil
}
