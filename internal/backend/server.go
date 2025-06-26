// internal/backend/server.go
package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// StartMockServer starts an HTTP server on the given address (e.g. ":0").
// It returns the server instance and the actual listening address.
func StartMockServer(addr string) (*http.Server, string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/stream", func(w http.ResponseWriter, r *http.Request) {
		var req BackendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		mode := os.Getenv("BACKEND_MODE")

		full := fmt.Sprintf("Echo: %s", req.Query)

		if mode == "sse" {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming unsupported", http.StatusInternalServerError)
				return
			}

			chunks := strings.Split(full, " ")
			for i, word := range chunks {
				event := fmt.Sprintf("id: %d\nevent: message_part\ndata: {\"text_chunk\": \"%s \"}\n\n", i, word)
				if _, err := w.Write([]byte(event)); err != nil {
					log.Printf("write SSE chunk error: %v", err)
					return
				}
				flusher.Flush()
				time.Sleep(250 * time.Millisecond)
			}
			endEvent := "id: done\nevent: stream_end\ndata: {\"status\": \"done\"}\n\n"
			if _, err := w.Write([]byte(endEvent)); err != nil {
				log.Printf("write SSE end error: %v", err)
			}
			return
		}

		// JSON mode
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"full_response": full}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("write JSON error: %v", err)
		}
	})

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Mock backend listen error: %v", err)
	}
	server := &http.Server{Handler: mux}
	go func() {
		log.Printf("Mock backend listening on %s (mode=%s)", ln.Addr().String(), os.Getenv("BACKEND_MODE"))
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Mock backend error: %v", err)
		}
	}()

	return server, ln.Addr().String()
}
