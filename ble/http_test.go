package ble

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordingWriter struct {
	mu     sync.Mutex
	writes [][]byte
	write  func([]byte) error
}

func (w *recordingWriter) Write(_ context.Context, data []byte) error {
	w.mu.Lock()
	w.writes = append(w.writes, bytes.Clone(data))
	w.mu.Unlock()
	if w.write != nil {
		return w.write(data)
	}
	return nil
}

func (w *recordingWriter) joined() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return bytes.Join(w.writes, nil)
}

func (w *recordingWriter) writeCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.writes)
}

func TestHTTPTransportSerializesDeleteBodyAndFragmentsWrites(t *testing.T) {
	var transport *httpTransport
	writer := new(recordingWriter)
	writer.write = func([]byte) error {
		if bytes.HasSuffix(writer.joined(), []byte(`{"element_ids":[1]}`)) {
			transport.handleNotification([]byte("HTTP/1.1 204 No Content\r\nContent-Length: 0\r\n\r\n"))
		}
		return nil
	}
	var err error
	transport, err = newHTTPTransport(writer, 17, 1024)
	if err != nil {
		t.Fatalf("newHTTPTransport: %v", err)
	}

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodDelete,
		"http://busybar.ble.invalid/api/display/draw?application_name=test",
		strings.NewReader(`{"element_ids":[1]}`),
	)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer checkCloseResponse(t, response)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("StatusCode = %d, want %d", response.StatusCode, http.StatusNoContent)
	}

	want := "DELETE /api/display/draw?application_name=test HTTP/1.1\r\n" +
		"Host: busybar.ble.invalid\r\n" +
		"Content-Length: 19\r\n" +
		"Connection: close\r\n" +
		"Content-Type: application/json\r\n" +
		"\r\n" +
		`{"element_ids":[1]}`
	if got := string(writer.joined()); got != want {
		t.Fatalf("raw request:\n%s\nwant:\n%s", got, want)
	}
	for index, fragment := range writer.writes {
		if len(fragment) > 17 {
			t.Fatalf("fragment %d has %d bytes, want at most 17", index, len(fragment))
		}
	}
}

func TestHTTPTransportRejectsAuthenticationHeadersBeforeWriting(t *testing.T) {
	writer := new(recordingWriter)
	transport, err := newHTTPTransport(writer, 20, 1024)
	if err != nil {
		t.Fatalf("newHTTPTransport: %v", err)
	}
	request, err := http.NewRequest(http.MethodGet, "http://busybar.ble.invalid/api/version", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	request.Header.Set("X-API-Token", "secret")

	response, err := transport.RoundTrip(request)
	if response != nil {
		checkCloseResponse(t, response)
	}
	if !errors.Is(err, errSensitiveHeader) {
		t.Fatalf("RoundTrip error = %v, want sensitive-header rejection", err)
	}
	if len(writer.writes) != 0 {
		t.Fatalf("writes = %d, want 0", len(writer.writes))
	}
}

func TestSerializeRequestAcceptsUnknownBodyLength(t *testing.T) {
	request, err := http.NewRequest(
		http.MethodPost,
		"http://busybar.ble.invalid/api/assets",
		io.NopCloser(strings.NewReader("payload")),
	)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if request.ContentLength != 0 {
		t.Fatalf("ContentLength = %d, want unknown zero length", request.ContentLength)
	}

	raw, err := serializeRequest(request, 1024)
	if err != nil {
		t.Fatalf("serializeRequest: %v", err)
	}
	if !bytes.Contains(raw, []byte("Content-Length: 7\r\n")) {
		t.Fatalf("serialized request does not contain computed Content-Length:\n%s", raw)
	}
	if !bytes.HasSuffix(raw, []byte("\r\npayload")) {
		t.Fatalf("serialized request does not preserve body:\n%s", raw)
	}
}

func TestHTTPTransportMarksFailureAfterWriteAsOutcomeUnknown(t *testing.T) {
	cause := errors.New("write callback failed")
	writer := &recordingWriter{write: func([]byte) error { return cause }}
	transport, err := newHTTPTransport(writer, 20, 1024)
	if err != nil {
		t.Fatalf("newHTTPTransport: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, "http://busybar.ble.invalid/api/input?key=confirm", http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	response, err := transport.RoundTrip(request)
	if response != nil {
		checkCloseResponse(t, response)
	}
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("RoundTrip error = %v, want ErrOutcomeUnknown", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("RoundTrip error = %v, want wrapped cause", err)
	}
}

func TestHTTPTransportRejectsRequestsAfterUnknownOutcome(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	requestWritten := make(chan struct{})
	var transport *httpTransport
	writer := new(recordingWriter)
	writer.write = func([]byte) error {
		if writer.writeCount() == 1 {
			close(requestWritten)
			return nil
		}
		transport.handleNotification([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}"))
		return nil
	}
	var err error
	transport, err = newHTTPTransport(writer, 512, 1024)
	if err != nil {
		t.Fatalf("newHTTPTransport: %v", err)
	}

	first, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://busybar.ble.invalid/api/input?key=confirm", http.NoBody)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	result := make(chan error, 1)
	go func() {
		response, err := transport.RoundTrip(first)
		if response != nil {
			err = errors.Join(err, response.Body.Close())
		}
		result <- err
	}()
	select {
	case <-requestWritten:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first request write")
	}
	cancel()
	if err := <-result; !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("first RoundTrip error = %v, want ErrOutcomeUnknown", err)
	}

	second, err := http.NewRequest(http.MethodGet, "http://busybar.ble.invalid/api/version", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	response, err := transport.RoundTrip(second)
	if response != nil {
		checkCloseResponse(t, response)
	}
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("second RoundTrip error = %v, want poisoned transport", err)
	}
	if got := writer.writeCount(); got != 1 {
		t.Fatalf("writes = %d, want no write after unknown outcome", got)
	}
}

func TestParseHTTPResponseRequiresExactBoundedContentLength(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "http://busybar.ble.invalid/api/version", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	tests := []struct {
		name string
		raw  string
		max  int
		want error
	}{
		{name: "missing", raw: "HTTP/1.1 200 OK\r\n\r\n{}", max: 1024, want: ErrProtocol},
		{name: "truncated", raw: "HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\n{}", max: 1024, want: nil},
		{name: "surplus", raw: "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}x", max: 1024, want: ErrProtocol},
		{name: "too large", raw: "HTTP/1.1 200 OK\r\nContent-Length: 8\r\n\r\n12345678", max: 45, want: ErrMessageTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, complete, err := parseHTTPResponse([]byte(test.raw), request, test.max)
			if response != nil {
				defer checkCloseResponse(t, response)
			}
			if test.name == "truncated" {
				if err != nil || complete {
					t.Fatalf("parseHTTPResponse = complete %t, error %v; want incomplete without error", complete, err)
				}
				return
			}
			if !errors.Is(err, test.want) {
				t.Fatalf("parseHTTPResponse error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestHTTPTransportReturnsCompleteResponseBody(t *testing.T) {
	var transport *httpTransport
	writer := &recordingWriter{write: func([]byte) error {
		transport.handleNotification([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}"))
		return nil
	}}
	var err error
	transport, err = newHTTPTransport(writer, 512, 1024)
	if err != nil {
		t.Fatalf("newHTTPTransport: %v", err)
	}
	request, err := http.NewRequest(http.MethodGet, "http://busybar.ble.invalid/api/version", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer checkCloseResponse(t, response)
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(body) != "{}" {
		t.Fatalf("body = %q, want %q", body, "{}")
	}
}

func TestHTTPTransportReconnectDoesNotResurrectInterruptedRequest(t *testing.T) {
	written := make(chan struct{})
	writer := &recordingWriter{write: func([]byte) error {
		select {
		case <-written:
		default:
			close(written)
		}
		return nil
	}}
	transport, err := newHTTPTransport(writer, 512, 1024)
	if err != nil {
		t.Fatalf("newHTTPTransport: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://busybar.ble.invalid/api/input?key=confirm", http.NoBody)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	result := make(chan error, 1)
	go func() {
		response, err := transport.RoundTrip(request)
		if response != nil {
			err = errors.Join(err, response.Body.Close())
		}
		result <- err
	}()
	select {
	case <-written:
	case <-ctx.Done():
		t.Fatal("timed out waiting for request write")
	}
	cause := errors.New("link lost")
	transport.handleDisconnect(cause)
	transport.handleReconnect()
	select {
	case err := <-result:
		if !errors.Is(err, ErrOutcomeUnknown) || !errors.Is(err, cause) {
			t.Fatalf("RoundTrip error = %v, want unknown outcome wrapping disconnect", err)
		}
	case <-ctx.Done():
		t.Fatal("interrupted request survived reconnect")
	}
}

func checkCloseResponse(t *testing.T, response *http.Response) {
	t.Helper()
	if err := response.Body.Close(); err != nil {
		t.Errorf("close response body: %v", err)
	}
}
