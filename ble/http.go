package ble

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/textproto"
	"slices"
	"strings"
	"sync"
)

type fragmentWriter interface {
	Write(context.Context, []byte) error
}

// httpTransport serializes requests because NUS response bytes contain no
// request correlation. An incomplete response after a write poisons the
// transport so late bytes cannot be assigned to a later request.
type httpTransport struct {
	writer        fragmentWriter
	fragmentLimit int
	maximum       int

	requestMu  sync.Mutex
	stateMu    sync.Mutex
	active     bool
	response   []byte
	terminal   error
	disconnect error
	desync     error
	closed     bool
	wake       chan struct{}
}

func newHTTPTransport(writer fragmentWriter, fragmentLimit int, maximum int64) (*httpTransport, error) {
	if writer == nil {
		return nil, errors.New("BLE fragment writer must not be nil")
	}
	if fragmentLimit <= 0 {
		return nil, errors.New("BLE fragment limit must be greater than zero")
	}
	if maximum <= 0 || maximum > int64(maxInt()) {
		return nil, errors.New("BLE message limit is outside the supported range")
	}
	return &httpTransport{writer: writer, fragmentLimit: fragmentLimit, maximum: int(maximum)}, nil
}

func (t *httpTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil {
		return nil, errors.New("BLE HTTP request must not be nil")
	}
	raw, err := serializeRequest(request, t.maximum)
	if err != nil {
		return nil, err
	}

	t.requestMu.Lock()
	defer t.requestMu.Unlock()
	if err := request.Context().Err(); err != nil {
		return nil, err
	}
	if err := t.available(); err != nil {
		return nil, err
	}

	t.beginResponse()
	defer t.endResponse()
	wrote := false
	for offset := 0; offset < len(raw); offset += t.fragmentLimit {
		end := min(offset+t.fragmentLimit, len(raw))
		wrote = true
		if err := t.writer.Write(request.Context(), raw[offset:end]); err != nil {
			return nil, t.desynchronize(fmt.Errorf("write fragment at offset %d: %w", offset, err))
		}
	}

	for {
		rawResponse, wake, terminal, disconnect := t.snapshot()
		if terminal != nil {
			if wrote {
				return nil, t.desynchronize(terminal)
			}
			return nil, terminal
		}
		response, complete, err := parseHTTPResponse(rawResponse, request, t.maximum)
		if err != nil {
			if wrote {
				return nil, t.desynchronize(err)
			}
			return nil, err
		}
		if complete {
			return response, nil
		}
		if disconnect != nil {
			return nil, t.desynchronize(disconnect)
		}
		select {
		case <-request.Context().Done():
			return nil, t.desynchronize(request.Context().Err())
		case <-wake:
		}
	}
}

func (t *httpTransport) available() error {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	if t.closed {
		return ErrClosed
	}
	if t.desync != nil {
		return t.desync
	}
	if t.disconnect != nil {
		return t.disconnect
	}
	return nil
}

func (t *httpTransport) desynchronize(cause error) error {
	err := unknownOutcome(cause)
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	if t.desync == nil {
		t.desync = err
	}
	t.signalLocked()
	return t.desync
}

func (t *httpTransport) beginResponse() {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.active = true
	t.response = nil
	t.terminal = nil
	t.wake = make(chan struct{}, 1)
}

func (t *httpTransport) endResponse() {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.active = false
	t.response = nil
	t.terminal = nil
	t.wake = nil
}

func (t *httpTransport) snapshot() ([]byte, <-chan struct{}, error, error) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	return bytes.Clone(t.response), t.wake, t.terminal, t.disconnect
}

func (t *httpTransport) handleNotification(data []byte) {
	if len(data) == 0 {
		return
	}
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	if !t.active || t.terminal != nil {
		return
	}
	if len(data) > t.maximum-len(t.response) {
		t.terminal = ErrMessageTooLarge
		t.signalLocked()
		return
	}
	t.response = append(t.response, data...)
	t.signalLocked()
}

func (t *httpTransport) handleReceiveError(cause error) {
	if cause == nil {
		return
	}
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	if t.active && t.terminal == nil {
		t.terminal = cause
		t.signalLocked()
	}
}

func (t *httpTransport) handleDisconnect(cause error) {
	if cause == nil {
		cause = ErrDisconnected
	}
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.disconnect = cause
	if t.active && t.terminal == nil {
		t.terminal = cause
	}
	t.signalLocked()
}

func (t *httpTransport) handleReconnect() {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.disconnect = nil
}

func (t *httpTransport) Close() error {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.closed = true
	t.disconnect = ErrClosed
	t.signalLocked()
	return nil
}

func (t *httpTransport) signalLocked() {
	if t.wake == nil {
		return
	}
	select {
	case t.wake <- struct{}{}:
	default:
	}
}

func serializeRequest(request *http.Request, maximum int) ([]byte, error) {
	if request.URL == nil {
		return nil, errors.New("BLE HTTP request URL must not be nil")
	}
	if len(request.TransferEncoding) != 0 {
		return nil, fmt.Errorf("%w: transfer encoding is unsupported", ErrProtocol)
	}

	var body []byte
	switch request.Method {
	case http.MethodGet:
		if request.Body != nil && request.Body != http.NoBody || request.ContentLength > 0 {
			return nil, fmt.Errorf("%w: GET request body is unsupported", ErrProtocol)
		}
	case http.MethodPost, http.MethodPut, http.MethodDelete:
		if request.Body != nil && request.Body != http.NoBody {
			var err error
			body, err = readRequestBody(request.Body, maximum)
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("%w: HTTP method %q is unsupported", ErrProtocol, request.Method)
	}
	if request.ContentLength > 0 && request.ContentLength != int64(len(body)) {
		return nil, fmt.Errorf("%w: request body is %d bytes but declares %d", ErrProtocol, len(body), request.ContentLength)
	}

	host := request.Host
	if host == "" {
		host = request.URL.Host
	}
	if host == "" || containsNewline(host) {
		return nil, fmt.Errorf("%w: invalid HTTP host", ErrProtocol)
	}

	var raw bytes.Buffer
	fmt.Fprintf(&raw, "%s %s HTTP/1.1\r\n", request.Method, request.URL.RequestURI())
	fmt.Fprintf(&raw, "Host: %s\r\n", host)
	fmt.Fprintf(&raw, "Content-Length: %d\r\n", len(body))
	raw.WriteString("Connection: close\r\n")
	for _, key := range slices.Sorted(maps.Keys(request.Header)) {
		canonical := textproto.CanonicalMIMEHeaderKey(key)
		if canonical == "" || containsNewline(key) {
			return nil, fmt.Errorf("%w: invalid HTTP header name", ErrProtocol)
		}
		switch strings.ToLower(canonical) {
		case "host", "content-length", "connection":
			continue
		case "authorization", "proxy-authorization", "x-api-token":
			return nil, errSensitiveHeader
		}
		for _, value := range request.Header.Values(key) {
			if containsNewline(value) {
				return nil, fmt.Errorf("%w: invalid HTTP header value", ErrProtocol)
			}
			fmt.Fprintf(&raw, "%s: %s\r\n", canonical, value)
		}
	}
	raw.WriteString("\r\n")
	raw.Write(body)
	if raw.Len() > maximum {
		return nil, ErrMessageTooLarge
	}
	return raw.Bytes(), nil
}

func readRequestBody(body io.ReadCloser, maximum int) ([]byte, error) {
	data, readErr := io.ReadAll(io.LimitReader(body, int64(maximum)+1))
	closeErr := body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, fmt.Errorf("read BLE request body: %w", err)
	}
	if len(data) > maximum {
		return nil, ErrMessageTooLarge
	}
	return data, nil
}

func parseHTTPResponse(raw []byte, request *http.Request, maximum int) (*http.Response, bool, error) {
	if len(raw) > maximum {
		return nil, false, ErrMessageTooLarge
	}
	headerEnd := bytes.Index(raw, []byte("\r\n\r\n"))
	if headerEnd < 0 {
		return nil, false, nil
	}
	response, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(raw)), request)
	if err != nil {
		return nil, false, fmt.Errorf("%w: %w", ErrProtocol, err)
	}
	if len(response.TransferEncoding) != 0 || response.ContentLength < 0 {
		_ = response.Body.Close()
		return nil, false, fmt.Errorf("%w: response requires an exact Content-Length", ErrProtocol)
	}
	bodyStart := headerEnd + 4
	if response.ContentLength > int64(maximum-bodyStart) {
		_ = response.Body.Close()
		return nil, false, ErrMessageTooLarge
	}
	expected := bodyStart + int(response.ContentLength)
	if len(raw) < expected {
		_ = response.Body.Close()
		return nil, false, nil
	}
	if len(raw) > expected {
		_ = response.Body.Close()
		return nil, false, fmt.Errorf("%w: response contains surplus bytes", ErrProtocol)
	}
	body := bytes.Clone(raw[bodyStart:expected])
	_ = response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(body))
	return response, true, nil
}

func unknownOutcome(err error) error {
	return errors.Join(ErrOutcomeUnknown, err)
}

func containsNewline(value string) bool { return strings.ContainsAny(value, "\r\n") }

func maxInt() int { return int(^uint(0) >> 1) }
