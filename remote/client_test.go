package remote

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	busylib "github.com/lxdb/busylib-go"
)

func TestNewClientValidatesFirmwareTopicSegmentsAndOptions(t *testing.T) {
	transport := newFakeTransport()
	tests := []struct {
		name      string
		transport Transport
		sessionID string
		options   []Option
	}{
		{name: "nil transport", sessionID: "session"},
		{name: "empty session", transport: transport},
		{name: "wildcard session", transport: transport, sessionID: "bad/+"},
		{name: "wildcard client", transport: transport, sessionID: "session", options: []Option{WithClientID("bad/#")}},
		{name: "zero timeout", transport: transport, sessionID: "session", options: []Option{WithRequestTimeout(0)}},
		{name: "fractional lease", transport: transport, sessionID: "session", options: []Option{WithStreamLease(1500 * time.Millisecond)}},
		{name: "incomplete message limit", transport: transport, sessionID: "session", options: []Option{WithStreamMessageLimit(MessageLimit{MaxCount: 1})}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewClient(test.transport, test.sessionID, test.options...); err == nil {
				t.Fatal("NewClient succeeded")
			}
		})
	}
}

func TestDeviceRoundTripUsesFirmwareHTTPTopicAndCorrelation(t *testing.T) {
	transport := newFakeTransport()
	transport.onPublish = func(message Message) {
		request, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(message.Payload)))
		if err != nil {
			t.Errorf("read HTTP request: %v", err)
			return
		}
		if request.Method != http.MethodGet || request.URL.RequestURI() != "/api/version?source=remote" {
			t.Errorf("request = %s %s", request.Method, request.URL.RequestURI())
		}
		if request.Header.Get("Authorization") != "" || request.Header.Get("X-API-Token") != "" {
			t.Errorf("unexpected auth headers: %#v", request.Header)
		}
		transport.deliver(Message{
			Topic:   message.Properties.ResponseTopic,
			Payload: []byte("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 23\r\n\r\n{\"api_semver\":\"25.0.0\"}"),
			QoS:     QoSAtLeastOnce,
			Properties: Properties{
				CorrelationData: append([]byte(nil), message.Properties.CorrelationData...),
			},
		})
	}

	client, err := NewClient(transport, "firmware-session", WithClientID("client-a"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	response, err := client.Device().Do(context.Background(), busylib.Request{
		Method:       http.MethodGet,
		Path:         "/api/version",
		Query:        mapValues("source", "remote"),
		ResponseMode: busylib.ResponseModeJSON,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if response.StatusCode != http.StatusOK || string(response.Body) != `{"api_semver":"25.0.0"}` {
		t.Fatalf("response = %#v", response)
	}

	published := transport.publishedMessages()
	if len(published) != 1 {
		t.Fatalf("published messages = %d, want 1", len(published))
	}
	message := published[0]
	if message.Topic != "sessions/firmware-session/down/v1/http-request" || message.QoS != QoSExactlyOnce {
		t.Fatalf("request publication = %#v", message)
	}
	if message.Properties.ResponseTopic != "sessions/firmware-session/up/v1/http-response/client-a" || len(message.Properties.CorrelationData) != 32 {
		t.Fatalf("request properties = %#v", message.Properties)
	}
	if got := transport.subscriptionRequests(); !reflect.DeepEqual(got, []SubscriptionRequest{{Topic: "sessions/firmware-session/up/v1/http-response/client-a", QoS: QoSAtLeastOnce}}) {
		t.Fatalf("subscriptions = %#v", got)
	}
}

func TestDeviceConcurrentRequestsAreCorrelatedOutOfOrder(t *testing.T) {
	transport := newFakeTransport()
	client, err := NewClient(transport, "session", WithClientID("concurrent"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	type result struct {
		body string
		err  error
	}
	results := make(chan result, 2)
	for _, value := range []string{"one", "two"} {
		value := value
		go func() {
			response, err := client.Device().Do(context.Background(), busylib.Request{
				Method:       http.MethodGet,
				Path:         "/api/version",
				Query:        mapValues("value", value),
				ResponseMode: busylib.ResponseModeText,
			})
			if err != nil {
				results <- result{err: err}
				return
			}
			results <- result{body: string(response.Body)}
		}()
	}

	requests := transport.waitPublished(t, 2)
	for index := len(requests) - 1; index >= 0; index-- {
		request := requests[index]
		httpRequest, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(request.Payload)))
		if err != nil {
			t.Fatalf("read HTTP request: %v", err)
		}
		value := httpRequest.URL.Query().Get("value")
		body := []byte(value)
		transport.deliver(Message{
			Topic:   request.Properties.ResponseTopic,
			Payload: []byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Length: %d\r\n\r\n%s", len(body), body)),
			QoS:     QoSAtLeastOnce,
			Properties: Properties{
				CorrelationData: request.Properties.CorrelationData,
			},
		})
	}

	got := make([]string, 0, 2)
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatalf("request failed: %v", result.err)
		}
		got = append(got, result.body)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, []string{"one", "two"}) {
		t.Fatalf("responses = %#v", got)
	}
}

func TestCloseFailsPendingRequestsWithoutClosingCallerTransport(t *testing.T) {
	transport := newFakeTransport()
	client, err := NewClient(transport, "session", WithClientID("close"), WithRequestTimeout(time.Minute))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := client.Device().Do(context.Background(), busylib.Request{
			Method:       http.MethodGet,
			Path:         "/api/version",
			ResponseMode: busylib.ResponseModeText,
		})
		done <- err
	}()
	transport.waitPublished(t, 1)
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrClosed) {
			t.Fatalf("request error = %v, want ErrClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("pending request was not released")
	}
	if transport.closed {
		t.Fatal("caller-owned transport was closed")
	}
}

func TestHTTPSubscriptionFailureFailsPendingAndNextRequestResubscribes(t *testing.T) {
	transport := newFakeTransport()
	client, err := NewClient(transport, "session", WithClientID("resubscribe"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	firstDone := make(chan error, 1)
	go func() {
		_, err := client.Device().Do(context.Background(), busylib.Request{
			Method:       http.MethodGet,
			Path:         "/api/version",
			ResponseMode: busylib.ResponseModeText,
		})
		firstDone <- err
	}()
	transport.waitPublished(t, 1)
	responseTopic := "sessions/session/up/v1/http-response/resubscribe"
	transport.closeLatestSubscription(t, responseTopic)
	select {
	case err := <-firstDone:
		if err == nil {
			t.Fatal("pending request succeeded after subscription failure")
		}
	case <-time.After(time.Second):
		t.Fatal("pending request was not failed")
	}

	transport.onPublish = func(message Message) {
		transport.deliver(Message{
			Topic:   message.Properties.ResponseTopic,
			Payload: []byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok"),
			QoS:     QoSAtLeastOnce,
			Properties: Properties{
				CorrelationData: message.Properties.CorrelationData,
			},
		})
	}
	response, err := client.Device().Do(context.Background(), busylib.Request{
		Method:       http.MethodGet,
		Path:         "/api/version",
		ResponseMode: busylib.ResponseModeText,
	})
	if err != nil {
		t.Fatalf("request after subscription failure: %v", err)
	}
	if string(response.Body) != "ok" {
		t.Fatalf("response body = %q", response.Body)
	}
	if got := transport.subscriptionRequests(); len(got) != 2 || got[0] != got[1] {
		t.Fatalf("subscriptions = %#v", got)
	}
}

func TestHTTPShutdownPreventsPublicationAfterClose(t *testing.T) {
	transport := newFakeTransport()
	roundTripper := newHTTPRoundTripper(transport, "session", "shutdown")
	body := &blockingReadCloser{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	request, err := http.NewRequest(http.MethodPost, syntheticBaseURL+"/api/name", body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	requestDone := make(chan error, 1)
	go func() {
		_, err := roundTripper.RoundTrip(request)
		requestDone <- err
	}()
	<-body.started

	closeDone := make(chan error, 1)
	go func() { closeDone <- roundTripper.Close() }()
	select {
	case err := <-closeDone:
		t.Fatalf("Close returned before the in-flight request stopped: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(body.release)
	if err := <-requestDone; !errors.Is(err, ErrClosed) {
		t.Fatalf("RoundTrip error = %v, want ErrClosed", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := transport.publishedMessages(); len(got) != 0 {
		t.Fatalf("published messages after shutdown = %#v", got)
	}
}

func mapValues(key, value string) map[string][]string {
	return map[string][]string{key: {value}}
}

type fakeTransport struct {
	mu                   sync.Mutex
	subscribers          map[string][]*fakeSubscription
	requests             []SubscriptionRequest
	published            []Message
	publishCh            chan Message
	onPublish            func(Message)
	subscriptionCloseErr error
	closed               bool
}

type blockingReadCloser struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *blockingReadCloser) Read([]byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	<-r.release
	return 0, io.EOF
}

func (*blockingReadCloser) Close() error { return nil }

func newFakeTransport() *fakeTransport {
	return &fakeTransport{
		subscribers: make(map[string][]*fakeSubscription),
		publishCh:   make(chan Message, 32),
	}
}

func (f *fakeTransport) Publish(_ context.Context, message Message) error {
	message = cloneMessage(message)
	f.mu.Lock()
	f.published = append(f.published, message)
	hook := f.onPublish
	f.mu.Unlock()
	f.publishCh <- message
	if hook != nil {
		hook(message)
	}
	return nil
}

func (f *fakeTransport) Subscribe(_ context.Context, request SubscriptionRequest) (Subscription, error) {
	subscription := &fakeSubscription{messages: make(chan Message, 32), done: make(chan struct{}), closeErr: f.subscriptionCloseErr}
	f.mu.Lock()
	f.requests = append(f.requests, request)
	f.subscribers[request.Topic] = append(f.subscribers[request.Topic], subscription)
	f.mu.Unlock()
	return subscription, nil
}

func (f *fakeTransport) deliver(message Message) {
	f.mu.Lock()
	subscribers := append([]*fakeSubscription(nil), f.subscribers[message.Topic]...)
	f.mu.Unlock()
	for _, subscriber := range subscribers {
		select {
		case subscriber.messages <- cloneMessage(message):
		case <-subscriber.done:
		}
	}
}

func (f *fakeTransport) waitPublished(t *testing.T, count int) []Message {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		f.mu.Lock()
		if len(f.published) >= count {
			messages := append([]Message(nil), f.published[:count]...)
			f.mu.Unlock()
			return messages
		}
		f.mu.Unlock()
		select {
		case <-f.publishCh:
		case <-deadline:
			t.Fatalf("published messages = %d, want %d", len(f.publishedMessages()), count)
		}
	}
}

func (f *fakeTransport) publishedMessages() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Message(nil), f.published...)
}

func (f *fakeTransport) subscriptionRequests() []SubscriptionRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]SubscriptionRequest(nil), f.requests...)
}

type fakeSubscription struct {
	messages chan Message
	done     chan struct{}
	once     sync.Once
	closeErr error
}

func (s *fakeSubscription) Receive(ctx context.Context) (Message, error) {
	select {
	case message := <-s.messages:
		return message, nil
	case <-s.done:
		return Message{}, io.EOF
	case <-ctx.Done():
		return Message{}, ctx.Err()
	}
}

func (s *fakeSubscription) Close() error {
	s.once.Do(func() { close(s.done) })
	return s.closeErr
}

func cloneMessage(message Message) Message {
	message.Payload = append([]byte(nil), message.Payload...)
	message.Properties.CorrelationData = append([]byte(nil), message.Properties.CorrelationData...)
	return message
}
