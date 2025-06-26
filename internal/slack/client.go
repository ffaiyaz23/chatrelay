package slack

import (
	"context"
	"log"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Client holds our Socket Mode client so we can listen and send messages.
type Client struct {
	smClient *socketmode.Client
}

// New constructs a Client.
//   - botToken: your “xoxb-…” token from Slack
//   - appToken: your “xapp-…” token (Socket Mode token)
func New(botToken, appToken string) *Client {
	// Create the base Slack API client.
	//  • slack.OptionDebug(false): turn off extra debug logs
	//  • slack.OptionAppLevelToken(appToken): attach your Socket Mode token
	api := slack.New(
		botToken,
		slack.OptionDebug(false),
		slack.OptionAppLevelToken(appToken),
	)

	// Wrap that API client with Socket Mode capabilities
	sm := socketmode.New(api)

	return &Client{smClient: sm}
}

// Run starts two things:
// 1. An event-loop goroutine that processes incoming Slack events.
// 2. The socketmode.Run() call, which keeps the WebSocket open.
func (c *Client) Run(ctx context.Context) {
	go c.runEventLoop() // start handling events in background
	log.Println("Connecting to Slack via Socket Mode…")
	c.smClient.Run() // this blocks until the connection closes
}

// runEventLoop reads events off c.smClient.Events (a Go channel)
// and dispatches only the AppMention events to our handler.
func (c *Client) runEventLoop() {
	for evt := range c.smClient.Events {
		log.Printf("%#v\n", evt.Type)
		// We only care about Events API payloads
		if evt.Type != socketmode.EventTypeEventsAPI {
			continue
		}

		// Try to cast evt.Data into the correct type
		eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
		if !ok {
			log.Printf("Ignored non-EventsAPI event: %#v", evt.Data)
			continue
		}

		// Acknowledge receipt of the event (Slack requires this)
		c.smClient.Ack(*evt.Request)

		// We're only handling callback events
		if eventsAPIEvent.Type != slackevents.CallbackEvent {
			continue
		}

		// Drill into the inner event; look for AppMentionEvent
		switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			c.handleAppMention(ev)
			// You could also add: case *slackevents.MessageEvent: // for DMs
		}
	}
}

// handleAppMention replies with a simple message when the bot is mentioned.
func (c *Client) handleAppMention(ev *slackevents.AppMentionEvent) {
	channel := ev.Channel // where to send the reply
	_, _, err := c.smClient.Client.PostMessage(
		channel,
		slack.MsgOptionText("Working on it…", false),
	)
	if err != nil {
		log.Printf("Error sending echo: %v", err)
	}
}
