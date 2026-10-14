package database

import (
	"fmt"
	"time"

	"github.com/gaucho-racing/notify/notify/config"
	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

var dbRetries = 0

func Init() {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		config.DatabaseHost, config.DatabaseUser, config.DatabasePassword, config.DatabaseName, config.DatabasePort)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		if dbRetries < 5 {
			dbRetries++
			logger.SugarLogger.Errorln("failed to connect database, retrying in 5s... ")
			time.Sleep(time.Second * 5)
			Init()
			return
		}
		logger.SugarLogger.Fatalf("failed to connect database after 5 attempts")
		return
	}

	logger.SugarLogger.Infoln("Connected to database")
	if err := db.AutoMigrate(
		&model.AppPolicy{},
		&model.Message{},
		&model.Delivery{},
		&model.DiscordBot{},
		&model.EmailSender{},
		&model.ResendAccount{},
	); err != nil {
		logger.SugarLogger.Fatalf("failed to run database migrations: %v", err)
		return
	}
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_notify_delivery_queue ON notify_delivery (next_attempt_at) WHERE status = 'PENDING'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_notify_discord_bot_default ON notify_discord_bot (is_default) WHERE is_default`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_notify_email_sender_default ON notify_email_sender (is_default) WHERE is_default`,
		`CREATE INDEX IF NOT EXISTS idx_notify_delivery_stats ON notify_delivery (client_id, created_at, channel, status)`,
	}
	for _, statement := range indexes {
		if err := db.Exec(statement).Error; err != nil {
			logger.SugarLogger.Fatalf("failed to create index: %v", err)
			return
		}
	}
	logger.SugarLogger.Infoln("AutoMigration complete")
	DB = db
}
