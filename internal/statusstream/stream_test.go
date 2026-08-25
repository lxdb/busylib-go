package statusstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
)

func TestStopTreatsAlreadyClosedConnectionAsStopped(t *testing.T) {
	accepted := make(chan struct{})
	release := make(chan struct{})
	serverErrors := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			serverErrors <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()
		close(accepted)
		<-release
	}))
	defer server.Close()
	defer close(release)

	conn, response, err := websocket.Dial(
		context.Background(),
		"ws"+strings.TrimPrefix(server.URL, "http"),
		nil,
	)
	if response != nil && response.Body != nil {
		defer func() { _ = response.Body.Close() }()
	}
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	<-accepted
	if err := conn.CloseNow(); err != nil {
		t.Fatalf("initial CloseNow: %v", err)
	}

	done := make(chan struct{})
	close(done)
	stream := &Stream{
		started: true,
		conn:    conn,
		done:    done,
	}

	if err := stream.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	select {
	case err := <-serverErrors:
		t.Fatalf("server error: %v", err)
	default:
	}
}
