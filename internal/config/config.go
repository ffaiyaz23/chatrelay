// internal/config/config.go
package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken       string
	AppToken       string
	SigningSecret  string
	BackendURL     string
	BackendMode    string // "sse" or "json"
	StreamMode     string // "update" or "thread"
	WorkerPoolSize string
}

func Load() Config {
	_ = godotenv.Load()
	cfg := Config{
		BotToken:       os.Getenv("SLACK_BOT_TOKEN"),
		AppToken:       os.Getenv("SLACK_APP_TOKEN"),
		SigningSecret:  os.Getenv("SLACK_SIGNING_SECRET"),
		BackendURL:     os.Getenv("BACKEND_URL"),
		BackendMode:    os.Getenv("BACKEND_MODE"),
		StreamMode:     os.Getenv("SLACK_STREAM_MODE"),
		WorkerPoolSize: os.Getenv("WORKER_POOL_SIZE"),
	}
	if cfg.BotToken == "" || cfg.AppToken == "" {
		log.Fatal("SLACK_BOT_TOKEN and SLACK_APP_TOKEN must be set")
	}
	if cfg.BackendMode == "" {
		cfg.BackendMode = "sse"
	}
	if cfg.StreamMode != "thread" {
		cfg.StreamMode = "update"
	}
	if cfg.WorkerPoolSize == "" {
		cfg.WorkerPoolSize = "10"
	}
	return cfg
}
