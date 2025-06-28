package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ffaiyaz23/chatrelay/internal/backend"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// workItem is a single mention to process.
type workItem struct {
	channel string
	ts      string
	user    string
	query   string
}

// updateItem is a chunk (plus final flag) to post back.
type updateItem struct {
	channel string
	ts      string
	text    string
	final   bool
}

// boolToInt helps record a boolean as an int attribute.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

var tracer = otel.Tracer("chatrelay")

// Client orchestrates dispatcher → worker pool → poster.
type Client struct {
	api           *slack.Client
	backendClient *backend.Client
	workCh        chan workItem
	updateCh      chan updateItem
	poolSize      int
	streamMode    string
}

// New constructs the HTTP-based Slack client pipeline.
func New(botToken string, backendClient *backend.Client, poolSize int, streamMode string) *Client {
	return &Client{
		api:           slack.New(botToken),
		backendClient: backendClient,
		workCh:        make(chan workItem, poolSize),
		updateCh:      make(chan updateItem, poolSize*2),
		poolSize:      poolSize,
		streamMode:    streamMode,
	}
}

// handleAppMention enqueues an AppMentionEvent into the pipeline with a tracing span
// and logs via zap, automatically including trace_id and span_id.
func (c *Client) handleAppMention(ctx context.Context, ev *slackevents.AppMentionEvent) {
	// 1) Start a span for this request
	ctx, span := tracer.Start(ctx, "ProcessAppMention",
		trace.WithAttributes(
			attribute.String("slack.user_id", ev.User),
			attribute.String("slack.channel_id", ev.Channel),
		),
	)
	defer span.End()

	// 2) Clean up the query text
	parts := strings.Fields(ev.Text)
	query := ev.Text
	if len(parts) > 1 && strings.HasPrefix(parts[0], "<@") {
		query = strings.Join(parts[1:], " ")
	}

	// 3) Post placeholder and log
	channelID, ts, err := c.api.PostMessage(
		ev.Channel,
		slack.MsgOptionText("🤖 Thinking…", false),
	)
	if err != nil {
		span.RecordError(err)
		logger := zap.L().With(
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.String("span_id", span.SpanContext().SpanID().String()),
		)
		logger.Error("failed to post placeholder", zap.Error(err))
		return
	}

	zap.S().Infow("enqueued work",
		"trace_id", span.SpanContext().TraceID().String(),
		"span_id", span.SpanContext().SpanID().String(),
		"channel", channelID,
		"ts", ts,
		"query", query,
	)

	// 4) Enqueue the work
	c.workCh <- workItem{channel: channelID, ts: ts, user: ev.User, query: query}
}

// startWorker pulls workItems, streams from the backend with its own span,
// and emits updateItems to be posted.
func (c *Client) startWorker(parentCtx context.Context) {
	for wi := range c.workCh {
		// a) Start a child span for the backend call
		ctx, span := tracer.Start(parentCtx, "CallBackend",
			trace.WithAttributes(attribute.String("backend.user_id", wi.user)),
		)
		chunks, err := c.backendClient.Stream(ctx, backend.BackendRequest{
			UserID: wi.user, Query: wi.query,
		})
		if err != nil {
			span.RecordError(err)
			span.End()
			zap.S().Errorw("backend stream error", "error", err)
			// enqueue a single final error update
			c.updateCh <- updateItem{channel: wi.channel, ts: wi.ts, text: "⚠ Backend error", final: true}
			continue
		}
		span.End()

		// b) Emit each chunk
		full := ""
		for chunk := range chunks {
			full += chunk
			c.updateCh <- updateItem{channel: wi.channel, ts: wi.ts, text: full, final: false}
		}
		// c) signal final update
		c.updateCh <- updateItem{channel: wi.channel, ts: wi.ts, text: full, final: true}
	}
}

// startPoster serializes updateItems back to Slack, tracing each API call
// and logging any errors.
func (c *Client) startPoster(ctx context.Context) {
	for ui := range c.updateCh {
		// start a span for posting this chunk
		_, span := tracer.Start(ctx, "PostSlackUpdate",
			trace.WithAttributes(attribute.Int("chunk_final", boolToInt(ui.final))),
		)

		if c.streamMode == "thread" {
			_, _, err := c.api.PostMessage(ui.channel,
				slack.MsgOptionText(ui.text, false),
				slack.MsgOptionTS(ui.ts),
			)
			if err != nil {
				span.RecordError(err)
				zap.S().Errorw("threaded post error", "error", err)
			}
		} else {
			_, _, _, err := c.api.UpdateMessage(ui.channel, ui.ts,
				slack.MsgOptionText(ui.text, false),
			)
			if err != nil {
				span.RecordError(err)
				zap.S().Errorw("message update error", "error", err)
			}
		}

		span.End()
		time.Sleep(50 * time.Millisecond) // smooth API calls
	}
}

// EventsHandler returns an HTTP handler that:
// 1) verifies Slack signatures,
// 2) handles URLVerification challenges,
// 3) parses AppMention callbacks,
// 4) and dispatches them into the same pipeline.
func EventsHandler(
	botToken, signingSecret string,
	poolSize int,
	streamMode, backendURL string,
) http.HandlerFunc {
	backendClient := backend.NewClient(backendURL)
	client := New(botToken, backendClient, poolSize, streamMode)

	// fire up the poster + worker pool
	ctx := context.Background()
	go client.startPoster(ctx)
	for i := 0; i < poolSize; i++ {
		go client.startWorker(ctx)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// 1) read full body
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body error", http.StatusBadRequest)
			return
		}
		// 2) verify Slack signature
		verifier, err := slack.NewSecretsVerifier(r.Header, signingSecret)
		if err != nil {
			http.Error(w, "signature init error", http.StatusInternalServerError)
			return
		}
		verifier.Write(raw)
		if err := verifier.Ensure(); err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		// 3) parse event
		evt, err := slackevents.ParseEvent(raw, slackevents.OptionNoVerifyToken())
		if err != nil {
			http.Error(w, "parse event error", http.StatusBadRequest)
			return
		}
		// 4) URL verification handshake
		if evt.Type == slackevents.URLVerification {
			var ch slackevents.ChallengeResponse
			_ = json.Unmarshal(raw, &ch)
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(ch.Challenge))
			return
		}
		// 5) dispatch AppMentionEvent
		if evt.Type == slackevents.CallbackEvent {
			if ev, ok := evt.InnerEvent.Data.(*slackevents.AppMentionEvent); ok {
				// use the request context so our spans & logs chain
				client.handleAppMention(r.Context(), ev)
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}
