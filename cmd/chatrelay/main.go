package main

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"github.com/ffaiyaz23/chatrelay/internal/backend"
	"github.com/ffaiyaz23/chatrelay/internal/config"
	"github.com/ffaiyaz23/chatrelay/internal/otel"
	"github.com/ffaiyaz23/chatrelay/internal/slack"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
)

func main() {
	// 0) Load configuration
	cfg := config.Load()

	// 1) Initialize OpenTelemetry tracing
	ctx := context.Background()
	tp, err := otel.InitTracer(ctx)
	if err != nil {
		// cannot use zap.L() yet, no logger; fallback to std log.Fatal
		panic("failed to init OTEL: " + err.Error())
	}
	defer func() { _ = tp.Shutdown(ctx) }()

	// 2) Initialize Zap logger and replace globals
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	// 3) Auto-start mock backend if BACKEND_URL is blank
	var backendURL string
	if cfg.BackendURL == "" {
		server, addr := backend.StartMockServer(":0")
		defer server.Close()
		backendURL = "http://" + addr
	} else {
		backendURL = cfg.BackendURL
	}
	zap.S().Infow("using backend", "url", backendURL, "mode", cfg.BackendMode)

	// 4) Worker pool size
	poolSize, _ := strconv.Atoi(cfg.WorkerPoolSize)

	// 5) Wire up the Events API endpoint (instrumented with otelhttp)
	eventsHandler := slack.EventsHandler(
		cfg.BotToken,
		cfg.SigningSecret,
		poolSize,
		cfg.StreamMode,
		backendURL,
	)
	http.Handle("/events", otelhttp.NewHandler(eventsHandler, "SlackEvents"))

	// 6) Start HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	zap.S().Infow("listening for Events API", "address", ":"+port+"/events")
	zap.S().Fatalw("HTTP server failed", "error", http.ListenAndServe(":"+port, nil))
}
