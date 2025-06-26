// internal/backend/client.go
package backend

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// Client calls the mock backend and streams text chunks.
type Client struct {
	BaseURL string
}

// NewClient creates a backend client pointing at baseURL (e.g. http://localhost:8080).
func NewClient(baseURL string) *Client {
	return &Client{BaseURL: baseURL}
}

// Stream sends the request and returns a channel of text chunks.
func (c *Client) Stream(ctx context.Context, req BackendRequest) (<-chan string, error) {
	out := make(chan string)
	jsonBody, _ := json.Marshal(req)
	r, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/chat/stream", strings.NewReader(string(jsonBody)))
	if err != nil {
		close(out)
		return nil, err
	}
	hresp, err := http.DefaultClient.Do(r)
	if err != nil {
		close(out)
		return nil, err
	}

	ct := hresp.Header.Get("Content-Type")
	go func() {
		defer hresp.Body.Close()
		if strings.HasPrefix(ct, "text/event-stream") {
			scanner := bufio.NewScanner(hresp.Body)
			currEvent := ""
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "event: ") {
					currEvent = strings.TrimSpace(line[len("event: "):])
					continue
				}
				if strings.HasPrefix(line, "data: ") {
					if currEvent != "message_part" {
						continue
					}
					var chunk struct {
						TextChunk string `json:"text_chunk"`
					}
					if err := json.Unmarshal([]byte(line[len("data: "):]), &chunk); err != nil {
						continue
					}
					if chunk.TextChunk != "" {
						out <- chunk.TextChunk
					}
				}
			}
		} else {
			var resp struct {
				Full string `json:"full_response"`
			}
			_ = json.NewDecoder(hresp.Body).Decode(&resp)
			words := strings.Fields(resp.Full)
			for _, w := range words {
				out <- w + " "
			}
		}
		close(out)
	}()
	return out, nil
}
