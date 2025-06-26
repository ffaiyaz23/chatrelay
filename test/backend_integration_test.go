// test/backend_integration_test.go
package backend_test

import (
	"context"
	"testing"
	"time"

	"github.com/ffaiyaz23/chatrelay/internal/backend"
)

func TestMockBackend_SSE(t *testing.T) {
	t.Setenv("BACKEND_MODE", "sse")
	server, addr := backend.StartMockServer(":0")
	defer server.Close()

	client := backend.NewClient("http://" + addr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := client.Stream(ctx, backend.BackendRequest{UserID: "U1", Query: "test sse"})
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	count := 0
	for chunk := range ch {
		if chunk == "" {
			t.Errorf("Empty chunk received")
		}
		count++
	}
	if count == 0 {
		t.Errorf("Expected >=1 chunks, got %d", count)
	}
}

func TestMockBackend_JSON(t *testing.T) {
	t.Setenv("BACKEND_MODE", "json")
	server, addr := backend.StartMockServer(":0")
	defer server.Close()

	client := backend.NewClient("http://" + addr)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ch, err := client.Stream(ctx, backend.BackendRequest{UserID: "U2", Query: "hello json"})
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	count := 0
	for range ch {
		count++
	}
	if count < 2 {
		t.Errorf("Expected multiple chunks, got %d", count)
	}
}
