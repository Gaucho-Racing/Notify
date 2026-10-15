package config

import "os"

const Name = "notify"
const Version = "1.0.1"

var Env = os.Getenv("ENV")
var Port = os.Getenv("PORT")

var DatabaseHost = os.Getenv("DATABASE_HOST")
var DatabasePort = os.Getenv("DATABASE_PORT")
var DatabaseUser = os.Getenv("DATABASE_USER")
var DatabasePassword = os.Getenv("DATABASE_PASSWORD")
var DatabaseName = os.Getenv("DATABASE_NAME")

var SentinelURL = os.Getenv("SENTINEL_URL")
var SentinelClientID = os.Getenv("SENTINEL_CLIENT_ID")
var SentinelClientSecret = os.Getenv("SENTINEL_CLIENT_SECRET")
var SentinelRedirectURI = os.Getenv("SENTINEL_REDIRECT_URI")
var SentinelSAToken = os.Getenv("SENTINEL_SA_TOKEN")

var EncryptionKey = os.Getenv("ENCRYPTION_KEY")

var WorkerConcurrency = os.Getenv("WORKER_CONCURRENCY")
var DeliveryMaxAttempts = os.Getenv("DELIVERY_MAX_ATTEMPTS")

func IsProduction() bool {
	return Env == "PROD"
}

func FormattedNameWithVersion() string {
	return Name + " v" + Version
}
