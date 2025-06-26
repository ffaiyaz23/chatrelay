package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all environment‐driven settings.
type Config struct {
	BotToken      string
	AppToken      string
	ClientID      string
	ClientSecret  string
	SigningSecret string

	BackendURL  string
	BackendMode string

	OTELExporter   string
	PrometheusPort string

	WorkerPoolSize      string
	GracefulTimeoutSecs string
}

// Load reads .env (if present) then the real env vars, or fatals if critical
func Load() Config {
	// Try to load a local .env file.
	// If it’s missing, we continue; OS env vars can still override.
	_ = godotenv.Load()

	cfg := Config{
		BotToken:      os.Getenv("SLACK_BOT_TOKEN"),
		AppToken:      os.Getenv("SLACK_APP_TOKEN"),
		ClientID:      os.Getenv("SLACK_CLIENT_ID"),
		ClientSecret:  os.Getenv("SLACK_CLIENT_SECRET"),
		SigningSecret: os.Getenv("SLACK_SIGNING_SECRET"),

		BackendURL:  os.Getenv("BACKEND_URL"),
		BackendMode: os.Getenv("BACKEND_MODE"),

		OTELExporter:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		PrometheusPort: os.Getenv("PROMETHEUS_PORT"),

		WorkerPoolSize:      os.Getenv("WORKER_POOL_SIZE"),
		GracefulTimeoutSecs: os.Getenv("GRACEFUL_TIMEOUT_SEC"),
	}

	// Validate the critical ones
	if cfg.BotToken == "" || cfg.AppToken == "" {
		log.Fatal("SLACK_BOT_TOKEN and SLACK_APP_TOKEN must be set (in your .env or environment)")
	}

	return cfg
}
