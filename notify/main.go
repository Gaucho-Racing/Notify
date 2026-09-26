package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/gaucho-racing/notify/notify/api"
	"github.com/gaucho-racing/notify/notify/config"
	"github.com/gaucho-racing/notify/notify/database"
	"github.com/gaucho-racing/notify/notify/pkg/logger"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
	"github.com/gaucho-racing/notify/notify/worker"
)

func main() {
	logger.Init(config.IsProduction())
	defer logger.Logger.Sync()

	config.Verify()
	config.PrintStartupBanner()
	sentinel.InitializeKeys()
	database.Init()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	workers := worker.Start(ctx)
	api.Run(ctx)
	workers.Wait()
	logger.SugarLogger.Infoln("Shutdown complete")
}
