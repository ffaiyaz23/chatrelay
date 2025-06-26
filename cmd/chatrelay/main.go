package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/ffaiyaz23/chatrelay/internal/backend"
	"github.com/ffaiyaz23/chatrelay/internal/config"
	"github.com/ffaiyaz23/chatrelay/internal/slack"
)

func main() {
	cfg := config.Load()

	// 1) Auto-start mock backend if none provided
	var backendURL string
	if cfg.BackendURL == "" {
		server, addr := backend.StartMockServer(":0")
		defer server.Close()
		backendURL = "http://" + addr
	} else {
		backendURL = cfg.BackendURL
	}
	log.Printf("Using backend at %s (mode=%s)", backendURL, cfg.BackendMode)

	// 2) Worker pool size
	poolSize, _ := strconv.Atoi(cfg.WorkerPoolSize)

	// 3) Wire up Events API endpoint, passing in the real backend URL
	http.HandleFunc("/events",
		slack.EventsHandler(
			cfg.BotToken,
			cfg.SigningSecret,
			poolSize,
			cfg.StreamMode,
			backendURL, // ← new parameter
		),
	)

	// 4) Start HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("Listening for Events API on :%s/events", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
