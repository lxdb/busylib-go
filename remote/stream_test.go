package remote

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lxdb/busylib-go/proto/statepb"
	publicstream "github.com/lxdb/busylib-go/stream"
	"google.golang.org/protobuf/proto"
)

func TestRemoteStatusStreamUsesFirmwareLeaseAndSharedDecoder(t *testing.T) {
	transport := newFakeTransport()
	client, err := NewClient(
		transport,
		"firmware-session",
		WithClientID("stream-client"),
		WithStreamLease(2*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	statusStream, err := client.NewStatusStream(publicstream.WithStaleAfter(100 * time.Millisecond))
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := statusStream.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	requests := transport.subscriptionRequests()
	wantSubscription := SubscriptionRequest{
		Topic: "sessions/firmware-session/up/v1/stream-response/stream-client",
		QoS:   QoSAtMostOnce,
	}
	if !reflect.DeepEqual(requests, []SubscriptionRequest{wantSubscription}) {
		t.Fatalf("subscriptions = %#v", requests)
	}
	start := transport.waitPublished(t, 1)[0]
	if start.Topic != "sessions/firmware-session/down/v1/stream-request" || start.QoS != QoSAtLeastOnce ||
		string(start.Payload) != "{}" || start.Properties.ResponseTopic != wantSubscription.Topic ||
		start.Properties.MessageExpiryIntervalSeconds == nil || *start.Properties.MessageExpiryIntervalSeconds != 2 {
		t.Fatalf("start publication = %#v", start)
	}

	payload, err := proto.Marshal(&statepb.State{
		Timestamp: 99,
		Updates: []*statepb.StateUpdate{
			{State: &statepb.StateUpdate_DeviceName{DeviceName: &statepb.DeviceName{Name: "remote"}}},
		},
	})
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	transport.deliver(Message{Topic: wantSubscription.Topic, Payload: payload, QoS: QoSAtMostOnce})
	select {
	case message := <-statusStream.Messages():
		if message.State.GetTimestamp() != 99 || len(message.Updates) != 1 || message.Updates[0].Kind() != publicstream.UpdateDeviceName {
			t.Fatalf("stream message = %#v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("status message was not delivered")
	}

	if err := statusStream.RequestSnapshot(context.Background()); !errors.Is(err, publicstream.ErrSnapshotUnsupported) {
		t.Fatalf("RequestSnapshot error = %v", err)
	}
	renewed := transport.waitPublished(t, 2)[1]
	if string(renewed.Payload) != "{}" || renewed.Properties.MessageExpiryIntervalSeconds == nil || *renewed.Properties.MessageExpiryIntervalSeconds != 2 {
		t.Fatalf("renewal publication = %#v", renewed)
	}

	if err := statusStream.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	published := transport.waitPublished(t, 3)
	if stop := published[2]; len(stop.Payload) != 0 || stop.QoS != QoSAtLeastOnce {
		t.Fatalf("stop publication = %#v", stop)
	}
	if status := statusStream.Status(); status.Lifecycle != publicstream.LifecycleStopped {
		t.Fatalf("status = %#v", status)
	}
}

func TestRemoteStatusStreamRateLimitAndSingleActiveStream(t *testing.T) {
	transport := newFakeTransport()
	client, err := NewClient(
		transport,
		"session",
		WithClientID("limited"),
		WithStreamMessageLimit(MessageLimit{MaxCount: 4, Interval: 1500 * time.Millisecond}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	first, err := client.NewStatusStream()
	if err != nil {
		t.Fatalf("first NewStatusStream: %v", err)
	}
	if _, err := client.NewStatusStream(); err == nil {
		t.Fatal("second active remote stream was accepted")
	}
	if err := first.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	start := transport.waitPublished(t, 1)[0]
	if string(start.Payload) != `{"message_limits":{"max_count":4,"interval_s":1.5}}` {
		t.Fatalf("rate-limit payload = %q", start.Payload)
	}
	if err := first.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	second, err := client.NewStatusStream()
	if err != nil {
		t.Fatalf("new stream after Stop: %v", err)
	}
	if err := second.Stop(); err != nil {
		t.Fatalf("stop idle stream: %v", err)
	}
}

func TestRemoteStatusStreamReconnectsAfterSubscriptionFailure(t *testing.T) {
	transport := newFakeTransport()
	client, err := NewClient(transport, "session", WithClientID("retry"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()
	statusStream, err := client.NewStatusStream(publicstream.WithReconnectPolicy(publicstream.ReconnectPolicy{MaxAttempts: 2, Delay: 0}))
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	if err := statusStream.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	transport.waitPublished(t, 1)
	transport.closeLatestSubscription(t, "sessions/session/up/v1/stream-response/retry")
	transport.waitSubscriptions(t, 2)
	transport.waitPublished(t, 2)
	if status := statusStream.Status(); status.Lifecycle != publicstream.LifecycleConnected || status.Attempt != 1 {
		t.Fatalf("status after reconnect = %#v", status)
	}
	if err := statusStream.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestClientCloseStopsActiveStreamAndLeavesTransportOpen(t *testing.T) {
	transport := newFakeTransport()
	client, err := NewClient(transport, "session", WithClientID("client-close"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	statusStream, err := client.NewStatusStream()
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	if err := statusStream.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	transport.waitPublished(t, 1)
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if status := statusStream.Status(); status.Lifecycle != publicstream.LifecycleStopped {
		t.Fatalf("stream status = %#v", status)
	}
	if transport.closed {
		t.Fatal("caller-owned transport was closed")
	}
	if _, err := client.NewStatusStream(); !errors.Is(err, ErrClosed) {
		t.Fatalf("NewStatusStream after close = %v", err)
	}
}

func TestConcurrentClientCloseWaitsForTheSameShutdown(t *testing.T) {
	transport := newFakeTransport()
	client, err := NewClient(transport, "session", WithClientID("concurrent-close"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	statusStream, err := client.NewStatusStream()
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	if err := statusStream.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	transport.waitPublished(t, 1)
	stopRelease := make(chan struct{})
	transport.mu.Lock()
	transport.onPublish = func(message Message) {
		if len(message.Payload) == 0 {
			<-stopRelease
		}
	}
	transport.mu.Unlock()

	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() { first <- client.Close() }()
	transport.waitPublished(t, 2)
	go func() { second <- client.Close() }()
	select {
	case err := <-second:
		t.Fatalf("concurrent Close returned before shutdown completed: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(stopRelease)
	if err := <-first; err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := <-second; err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestStatusStreamStopReturnsSubscriptionCloseError(t *testing.T) {
	closeErr := errors.New("unsubscribe failed")
	transport := newFakeTransport()
	transport.subscriptionCloseErr = closeErr
	client, err := NewClient(transport, "session", WithClientID("close-error"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	statusStream, err := client.NewStatusStream()
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	if err := statusStream.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	transport.waitPublished(t, 1)
	if err := statusStream.Stop(); !errors.Is(err, closeErr) {
		t.Fatalf("Stop error = %v, want %v", err, closeErr)
	}
	_ = client.Close()
}

func (f *fakeTransport) closeLatestSubscription(t *testing.T, topic string) {
	t.Helper()
	f.mu.Lock()
	subscriptions := f.subscribers[topic]
	f.mu.Unlock()
	if len(subscriptions) == 0 {
		t.Fatalf("no subscription for %s", topic)
	}
	if err := subscriptions[len(subscriptions)-1].Close(); err != nil {
		t.Fatalf("close subscription: %v", err)
	}
}

func (f *fakeTransport) waitSubscriptions(t *testing.T, count int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(f.subscriptionRequests()) >= count {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("subscriptions = %d, want %d", len(f.subscriptionRequests()), count)
}
