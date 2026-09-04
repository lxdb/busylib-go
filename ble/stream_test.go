package ble

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lxdb/busylib-go/proto/statepb"
	publicstream "github.com/lxdb/busylib-go/stream"
	"google.golang.org/protobuf/proto"
)

func TestStatusStreamDecodesFFE1State(t *testing.T) {
	connection := new(fakeConnection)
	client := connectedTestClient(t, connection)
	t.Cleanup(func() { checkCloseClient(t, client) })
	statusStream, err := client.NewStatusStream(publicstream.WithReconnectPolicy(publicstream.ReconnectPolicy{MaxAttempts: 1}))
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := statusStream.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	payload, err := proto.Marshal(&statepb.State{Timestamp: 1234})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	connection.emitState(statePacket(0, 1, payload))
	select {
	case message := <-statusStream.Messages():
		if message.State == nil || message.State.GetTimestamp() != 1234 {
			t.Fatalf("state = %+v, want timestamp 1234", message.State)
		}
		if message.Kind != publicstream.MessageBinary {
			t.Fatalf("kind = %q, want binary", message.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for decoded FFE1 state")
	}
	cancel()
	if err := statusStream.Wait(); err != nil {
		t.Fatalf("Wait after cancellation: %v", err)
	}
}

func TestStatusStreamRecordsConnectionTime(t *testing.T) {
	client := connectedTestClient(t, new(fakeConnection))
	t.Cleanup(func() { checkCloseClient(t, client) })
	statusStream, err := client.NewStatusStream()
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := statusStream.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if status := statusStream.Status(); status.ConnectedAt.IsZero() {
		t.Fatalf("connected status = %+v, want nonzero ConnectedAt", status)
	}
	cancel()
	if err := statusStream.Wait(); err != nil {
		t.Fatalf("Wait after cancellation: %v", err)
	}
}

func TestStatusStreamBecomesStaleWithoutFirstState(t *testing.T) {
	client := connectedTestClient(t, new(fakeConnection))
	t.Cleanup(func() { checkCloseClient(t, client) })
	statusStream, err := client.NewStatusStream(publicstream.WithStaleAfter(10 * time.Millisecond))
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := statusStream.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.After(time.Second)
	for statusStream.Status().Data != publicstream.DataStale {
		select {
		case <-statusStream.Statuses():
		case <-deadline:
			t.Fatalf("status = %+v, want DataStale without first state", statusStream.Status())
		}
	}
	cancel()
	if err := statusStream.Wait(); err != nil {
		t.Fatalf("Wait after cancellation: %v", err)
	}
}

func TestStatusStreamReconnectsAndRestoresStateNotifications(t *testing.T) {
	connection := &fakeConnection{
		reconnectStarted: make(chan struct{}),
		reconnectRelease: make(chan struct{}),
	}
	t.Cleanup(func() { closeIfOpen(connection.reconnectRelease) })
	client := connectedTestClient(t, connection)
	t.Cleanup(func() { checkCloseClient(t, client) })
	statusStream, err := client.NewStatusStream(publicstream.WithReconnectPolicy(publicstream.ReconnectPolicy{
		MaxAttempts: 2,
		Delay:       0,
	}))
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := statusStream.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForLifecycle(t, statusStream.Statuses(), publicstream.LifecycleConnected)
	connection.emitDisconnect(errors.New("link lost"))
	select {
	case <-connection.reconnectStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reconnect attempt")
	}

	waitForLifecycle(t, statusStream.Statuses(), publicstream.LifecycleReconnecting)
	closeIfOpen(connection.reconnectRelease)
	waitForLifecycle(t, statusStream.Statuses(), publicstream.LifecycleConnected)
	connection.mu.Lock()
	enables := connection.stateEnables
	reconnects := connection.reconnects
	connection.mu.Unlock()
	if reconnects != 1 || enables != 2 {
		t.Fatalf("reconnects = %d, state enables = %d; want 1 and 2", reconnects, enables)
	}
	cancel()
	if err := statusStream.Wait(); err != nil {
		t.Fatalf("Wait after cancellation: %v", err)
	}
}

func closeIfOpen(channel chan struct{}) {
	select {
	case <-channel:
	default:
		close(channel)
	}
}

func waitForLifecycle(t *testing.T, statuses <-chan publicstream.Status, want publicstream.Lifecycle) {
	t.Helper()
	for {
		select {
		case status := <-statuses:
			if status.Lifecycle == want {
				return
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for lifecycle %q", want)
		}
	}
}

func TestStatusStreamReturnsStableReconnectFailure(t *testing.T) {
	cause := errors.New("saved-link encryption failed")
	connection := &fakeConnection{reconnectErr: cause}
	client := connectedTestClient(t, connection)
	t.Cleanup(func() { checkCloseClient(t, client) })
	statusStream, err := client.NewStatusStream(publicstream.WithReconnectPolicy(publicstream.ReconnectPolicy{
		MaxAttempts: 2,
		Delay:       0,
	}))
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	if err := statusStream.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	connection.emitDisconnect(errors.New("link lost"))
	if err := statusStream.Wait(); !errors.Is(err, cause) {
		t.Fatalf("Wait error = %v, want reconnect cause", err)
	}
	if err := statusStream.Wait(); !errors.Is(err, cause) {
		t.Fatalf("second Wait error = %v, want stable reconnect cause", err)
	}
	connection.mu.Lock()
	reconnects := connection.reconnects
	connection.mu.Unlock()
	if reconnects != 2 {
		t.Fatalf("reconnects = %d, want 2", reconnects)
	}
}

func TestClientAllowsOnlyOneActiveStatusStream(t *testing.T) {
	client := connectedTestClient(t, new(fakeConnection))
	t.Cleanup(func() { checkCloseClient(t, client) })
	first, err := client.NewStatusStream()
	if err != nil {
		t.Fatalf("first NewStatusStream: %v", err)
	}
	if _, err := client.NewStatusStream(); err == nil {
		t.Fatal("second NewStatusStream succeeded, want sole-active-stream error")
	}
	if err := first.Stop(); err != nil {
		t.Fatalf("Stop before Start: %v", err)
	}
	if _, err := client.NewStatusStream(); err != nil {
		t.Fatalf("NewStatusStream after Stop: %v", err)
	}
}

func TestStatusStreamRejectsSnapshotRequests(t *testing.T) {
	client := connectedTestClient(t, new(fakeConnection))
	t.Cleanup(func() { checkCloseClient(t, client) })
	statusStream, err := client.NewStatusStream()
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	if err := statusStream.RequestSnapshot(context.Background()); !errors.Is(err, publicstream.ErrSnapshotUnsupported) {
		t.Fatalf("RequestSnapshot error = %v, want ErrSnapshotUnsupported", err)
	}
}

func connectedTestClient(t *testing.T, connection *fakeConnection) *Client {
	t.Helper()
	client, err := connect(
		context.Background(),
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		defaultConfig(),
		&fakeBackend{connection: connection},
	)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return client
}
