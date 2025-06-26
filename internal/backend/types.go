package backend

// BackendRequest defines the payload sent to /v1/chat/stream
// UserID is the Slack user that initiated the request.
// Query is the user’s input text.
type BackendRequest struct {
	UserID string `json:"user_id"`
	Query  string `json:"query"`
}
