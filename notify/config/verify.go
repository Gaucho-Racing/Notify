package config

import (
	"strconv"

	"github.com/gaucho-racing/notify/notify/pkg/logger"
	"github.com/gaucho-racing/notify/notify/pkg/secretbox"
)

var WorkerConcurrencyLimit int
var DeliveryMaxAttemptsLimit int

// Secrets stays nil without ENCRYPTION_KEY; every Seal and Open on it then
// returns secretbox.ErrNotConfigured.
var Secrets *secretbox.Box

func Verify() {
	if Env == "" {
		Env = "PROD"
		logger.SugarLogger.Infof("ENV is not set, defaulting to %s", Env)
	}
	if Port == "" {
		Port = "9999"
		logger.SugarLogger.Infof("PORT is not set, defaulting to %s", Port)
	}
	if DatabaseHost == "" {
		DatabaseHost = "localhost"
		logger.SugarLogger.Infof("DATABASE_HOST is not set, defaulting to %s", DatabaseHost)
	}
	if DatabasePort == "" {
		DatabasePort = "5432"
		logger.SugarLogger.Infof("DATABASE_PORT is not set, defaulting to %s", DatabasePort)
	}
	if DatabaseUser == "" {
		DatabaseUser = "postgres"
		logger.SugarLogger.Infof("DATABASE_USER is not set, defaulting to %s", DatabaseUser)
	}
	if DatabasePassword == "" {
		DatabasePassword = "password"
		logger.SugarLogger.Infof("DATABASE_PASSWORD is not set, defaulting to %s", DatabasePassword)
	}
	if DatabaseName == "" {
		DatabaseName = "notify"
		logger.SugarLogger.Infof("DATABASE_NAME is not set, defaulting to %s", DatabaseName)
	}
	if SentinelURL == "" {
		logger.SugarLogger.Fatal("SENTINEL_URL is required")
	}
	if SentinelClientID == "" || SentinelClientSecret == "" {
		logger.SugarLogger.Warnf("SENTINEL_CLIENT_ID / SENTINEL_CLIENT_SECRET are not set, web login will be unavailable and no token will resolve as first-party (admin access unreachable)")
	}
	if SentinelSAToken == "" {
		logger.SugarLogger.Warnf("SENTINEL_SA_TOKEN is not set, recipients cannot be resolved and every delivery will fail")
	}
	if ResendAPIKey == "" {
		logger.SugarLogger.Warnf("RESEND_API_KEY is not set, email deliveries will fail")
	}
	if EncryptionKey == "" {
		logger.SugarLogger.Warnf("ENCRYPTION_KEY is not set, discord bots cannot be configured and discord deliveries will fail")
	} else {
		box, err := secretbox.New(EncryptionKey)
		if err != nil {
			logger.SugarLogger.Fatalf("ENCRYPTION_KEY is invalid: %v", err)
		}
		Secrets = box
	}
	WorkerConcurrencyLimit = positiveInt("WORKER_CONCURRENCY", &WorkerConcurrency, "8")
	DeliveryMaxAttemptsLimit = positiveInt("DELIVERY_MAX_ATTEMPTS", &DeliveryMaxAttempts, "6")
}

func positiveInt(name string, value *string, fallback string) int {
	if *value == "" {
		*value = fallback
		logger.SugarLogger.Infof("%s is not set, defaulting to %s", name, fallback)
	}
	parsed, err := strconv.Atoi(*value)
	if err != nil || parsed <= 0 {
		logger.SugarLogger.Fatalf("%s is not a valid positive integer", name)
	}
	return parsed
}
