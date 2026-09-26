package main

import (
	"github.com/gaucho-racing/notify/notify/api"
	"github.com/gaucho-racing/notify/notify/config"
	"github.com/gaucho-racing/notify/notify/database"
	"github.com/gaucho-racing/notify/notify/pkg/logger"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
)

func main() {
	logger.Init(config.IsProduction())
	defer logger.Logger.Sync()

	config.Verify()
	config.PrintStartupBanner()
	sentinel.InitializeKeys()
	database.Init()

	api.Run()
}
