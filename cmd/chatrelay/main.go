package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ffaiyaz23/chatrelay/internal/config"
	slackclient "github.com/ffaiyaz23/chatrelay/internal/slack"
)

func main() {
	// Load tokens (will fatally exit if missing)
	cfg := config.Load()
	log.Printf("DEBUG: BotToken len=%d, AppToken len=%d\n", len(cfg.BotToken), len(cfg.AppToken))

	// Create Slack Socket Mode client
	client := slackclient.New(cfg.BotToken, cfg.AppToken)

	// Handle graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start the Slack client in a goroutine
	go client.Run(ctx)

	log.Println("ChatRelay is running. Press Ctrl+C to stop.")
	<-ctx.Done()
	log.Println("Shutting down...")
}
