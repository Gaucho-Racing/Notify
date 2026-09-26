package database

import (
	"fmt"
	"time"

	"github.com/gaucho-racing/notify/notify/config"
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
	if err := db.AutoMigrate(); err != nil {
		logger.SugarLogger.Fatalf("failed to run database migrations: %v", err)
		return
	}
	logger.SugarLogger.Infoln("AutoMigration complete")
	DB = db
}
