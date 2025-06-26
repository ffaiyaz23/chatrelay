package slack

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ffaiyaz23/chatrelay/internal/backend"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
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

// Client orchestrates dispatcher → worker pool → poster.
type Client struct {
	api           *slack.Client
	backendClient *backend.Client
	workCh        chan workItem
	updateCh      chan updateItem
	poolSize      int
	streamMode    string
}

// New constructs the HTTP‐based Slack client pipeline.
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

// handleAppMention enqueues an AppMentionEvent into the pipeline.
func (c *Client) handleAppMention(ev *slackevents.AppMentionEvent) {
	channelID := ev.Channel
	userID := ev.User
	parts := strings.Fields(ev.Text)
	query := ev.Text
	if len(parts) > 1 && strings.HasPrefix(parts[0], "<@") {
		query = strings.Join(parts[1:], " ")
	}

	// Post placeholder
	channelID, ts, err := c.api.PostMessage(
		channelID,
		slack.MsgOptionText("🤖 Thinking…", false),
	)
	if err != nil {
		log.Printf("post placeholder error: %v", err)
		return
	}
	log.Printf("Posted placeholder in %s at %s", channelID, ts)

	// Enqueue work
	c.workCh <- workItem{channel: channelID, ts: ts, user: userID, query: query}
}

// startWorker pulls workItems, streams from the backend, and emits updateItems.
func (c *Client) startWorker(ctx context.Context) {
	for wi := range c.workCh {
		chunks, err := c.backendClient.Stream(ctx, backend.BackendRequest{
			UserID: wi.user, Query: wi.query,
		})
		if err != nil {
			c.updateCh <- updateItem{wi.channel, wi.ts, "⚠ Backend error", true}
			continue
		}
		full := ""
		for chunk := range chunks {
			full += chunk
			c.updateCh <- updateItem{wi.channel, wi.ts, full, false}
		}
		// final
		c.updateCh <- updateItem{wi.channel, wi.ts, full, true}
	}
}

// startPoster reads updateItems and sends them back to Slack.
func (c *Client) startPoster(ctx context.Context) {
	for ui := range c.updateCh {
		if c.streamMode == "thread" {
			// threaded replies
			_, _, err := c.api.PostMessage(
				ui.channel,
				slack.MsgOptionText(ui.text, false),
				slack.MsgOptionTS(ui.ts),
			)
			if err != nil {
				log.Printf("thread post error: %v", err)
			}
		} else {
			// in-place update
			_, _, _, err := c.api.UpdateMessage(
				ui.channel, ui.ts,
				slack.MsgOptionText(ui.text, false),
			)
			if err != nil {
				log.Printf("update error: %v", err)
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// EventsHandler returns an HTTP handler that:
// • verifies Slack signatures,
// • handles URL verification challenges,
// • parses AppMention events,
// • and pipes them through the same Go pipeline.
func EventsHandler(botToken, signingSecret string, poolSize int, streamMode, backendURL string) http.HandlerFunc {
	backendClient := backend.NewClient(backendURL)

	client := New(botToken, backendClient, poolSize, streamMode)
	ctx := context.Background()
	go client.startPoster(ctx)
	for i := 0; i < poolSize; i++ {
		go client.startWorker(ctx)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// 1) Read body
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body error", http.StatusBadRequest)
			return
		}

		// 2) Verify signature
		sv, err := slack.NewSecretsVerifier(r.Header, signingSecret)
		if err != nil {
			http.Error(w, "signature init error", http.StatusInternalServerError)
			return
		}
		sv.Write(raw)
		if err := sv.Ensure(); err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		// 3) Parse event
		evt, err := slackevents.ParseEvent(raw, slackevents.OptionNoVerifyToken())
		if err != nil {
			http.Error(w, "parse event error", http.StatusBadRequest)
			return
		}

		// 4) URL verification challenge
		if evt.Type == slackevents.URLVerification {
			var ch slackevents.ChallengeResponse
			json.Unmarshal(raw, &ch)
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(ch.Challenge))
			return
		}

		// 5) AppMention callbacks
		if evt.Type == slackevents.CallbackEvent {
			if ev, ok := evt.InnerEvent.Data.(*slackevents.AppMentionEvent); ok {
				client.handleAppMention(ev)
			}
		}

		w.WriteHeader(http.StatusOK)
	}
}
