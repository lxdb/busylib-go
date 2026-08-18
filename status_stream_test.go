package busylib

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	framepkg "github.com/lxdb/busylib-go/frame"
	"github.com/lxdb/busylib-go/proto/errorpb"
	"github.com/lxdb/busylib-go/proto/framepb"
	"github.com/lxdb/busylib-go/proto/statepb"
	publicstream "github.com/lxdb/busylib-go/stream"
	"google.golang.org/protobuf/proto"
)

const streamTestTimeout = 3 * time.Second

type streamHandshake struct {
	token         string
	version       string
	tokenHeader   string
	versionHeader string
	control       string
}

func TestStatusStreamHandshakeMessagesAndSnapshotControl(t *testing.T) {
	handshakes := make(chan streamHandshake, 1)
	snapshots := make(chan string, 1)
	serverErrors := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			serverErrors <- err
			return
		}
		defer conn.CloseNow()

		messageType, control, err := streamRead(conn)
		if err != nil {
			serverErrors <- err
			return
		}
		if messageType != websocket.MessageText {
			serverErrors <- errors.New("initial stream control was not text")
			return
		}
		handshakes <- streamHandshake{
			token:         r.URL.Query().Get("x-api-token"),
			version:       r.URL.Query().Get("x-api-sem-ver"),
			tokenHeader:   r.Header.Get("X-API-Token"),
			versionHeader: r.Header.Get("X-API-Sem-Ver"),
			control:       string(control),
		}

		if err := streamWriteState(conn, &statepb.State{
			Timestamp: 7,
			Updates: []*statepb.StateUpdate{{
				State: &statepb.StateUpdate_DeviceName{DeviceName: &statepb.DeviceName{Name: "Desk"}},
			}},
		}); err != nil {
			serverErrors <- err
			return
		}
		if err := conn.Write(context.Background(), websocket.MessageText, []byte("ready")); err != nil {
			serverErrors <- err
			return
		}

		_, snapshot, err := streamRead(conn)
		if err != nil {
			serverErrors <- err
			return
		}
		snapshots <- string(snapshot)
	}))
	defer server.Close()

	statusStream := newTestStatusStream(t, server, WithLocalAccessKey("1234"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := statusStream.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	handshake := receiveStreamValue(t, handshakes)
	if handshake.token != "1234" || handshake.version != "25.0.0" {
		t.Fatalf("query token/version = %q/%q", handshake.token, handshake.version)
	}
	if handshake.tokenHeader != "" || handshake.versionHeader != "" {
		t.Fatalf("WebSocket auth/version headers = %q/%q, want absent", handshake.tokenHeader, handshake.versionHeader)
	}
	if handshake.control != `{"enable":true,"send":"all"}` {
		t.Fatalf("initial control = %q", handshake.control)
	}

	stateMessage := receiveStreamValue(t, statusStream.Messages())
	if stateMessage.Kind != publicstream.MessageBinary || stateMessage.State.GetTimestamp() != 7 {
		t.Fatalf("state message = %#v", stateMessage)
	}
	if len(stateMessage.Updates) != 1 || stateMessage.Updates[0].Kind() != publicstream.UpdateDeviceName {
		t.Fatalf("state updates = %#v", stateMessage.Updates)
	}
	textMessage := receiveStreamValue(t, statusStream.Messages())
	if textMessage.Kind != publicstream.MessageText || textMessage.Text != "ready" {
		t.Fatalf("text message = %#v", textMessage)
	}

	if err := statusStream.RequestSnapshot(context.Background()); err != nil {
		t.Fatalf("RequestSnapshot: %v", err)
	}
	if snapshot := receiveStreamValue(t, snapshots); snapshot != `{"send":"all"}` {
		t.Fatalf("snapshot control = %q", snapshot)
	}
	if err := statusStream.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	assertNoServerError(t, serverErrors)
}

func TestStatusStreamFrameDecodesToRGBA(t *testing.T) {
	hold := make(chan struct{})
	serverErrors := make(chan error, 1)
	raw := make([]byte, framepkg.FrontWidth*framepkg.FrontHeight*3)
	raw[0], raw[1], raw[2] = 0x11, 0x22, 0x33
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			serverErrors <- err
			return
		}
		defer conn.CloseNow()
		if _, _, err := streamRead(conn); err != nil {
			serverErrors <- err
			return
		}
		if err := streamWriteState(conn, &statepb.State{
			Timestamp: 8,
			Updates: []*statepb.StateUpdate{{
				State: &statepb.StateUpdate_Frame{Frame: &framepb.Frame{
					Screen:      framepb.Screen_FRONT,
					Width:       framepkg.FrontWidth,
					Height:      framepkg.FrontHeight,
					Encoding:    framepb.Encoding_PLAIN,
					PixelFormat: framepb.PixelFormat_RGB888,
					Data:        raw,
				}},
			}},
		}); err != nil {
			serverErrors <- err
			return
		}
		<-hold
	}))
	defer server.Close()
	defer close(hold)

	statusStream := newTestStatusStream(t, server)
	if err := statusStream.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	message := receiveStreamValue(t, statusStream.Messages())
	if len(message.Updates) != 1 {
		t.Fatalf("updates = %#v", message.Updates)
	}
	update, ok := message.Updates[0].(publicstream.FrameUpdate)
	if !ok {
		t.Fatalf("frame update type = %T", message.Updates[0])
	}
	value, err := framepkg.FromProto(update.Value)
	if err != nil {
		t.Fatalf("FromProto: %v", err)
	}
	rgba, err := value.RGBA()
	if err != nil {
		t.Fatalf("RGBA: %v", err)
	}
	if pixel := rgba.RGBAAt(0, 0); pixel.R != 0x33 || pixel.G != 0x22 || pixel.B != 0x11 || pixel.A != 0xff {
		t.Fatalf("first pixel = %#v", pixel)
	}
	if err := statusStream.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	assertNoServerError(t, serverErrors)
}

func TestStatusStreamStaleAndFreshRecovery(t *testing.T) {
	sendAgain := make(chan struct{})
	hold := make(chan struct{})
	serverErrors := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			serverErrors <- err
			return
		}
		defer conn.CloseNow()
		if _, _, err := streamRead(conn); err != nil {
			serverErrors <- err
			return
		}
		if err := streamWriteState(conn, &statepb.State{Timestamp: 1}); err != nil {
			serverErrors <- err
			return
		}
		<-sendAgain
		if err := streamWriteState(conn, &statepb.State{Timestamp: 2}); err != nil {
			serverErrors <- err
			return
		}
		<-hold
	}))
	defer server.Close()
	defer close(hold)

	statusStream := newTestStatusStreamWithOptions(t, server, publicstream.WithStaleAfter(50*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := statusStream.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	receiveStreamValue(t, statusStream.Messages())
	waitForStreamStatus(t, statusStream, func(status publicstream.Status) bool {
		return status.Data == publicstream.DataFresh
	})
	waitForStreamStatus(t, statusStream, func(status publicstream.Status) bool {
		return status.Data == publicstream.DataStale
	})
	close(sendAgain)
	receiveStreamValue(t, statusStream.Messages())
	waitForStreamStatus(t, statusStream, func(status publicstream.Status) bool {
		return status.Data == publicstream.DataFresh && !status.LastStateAt.IsZero()
	})
	if err := statusStream.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	assertNoServerError(t, serverErrors)
}

func TestStatusStreamReconnectsAndRequestsSnapshotAgain(t *testing.T) {
	var connections atomic.Int32
	controls := make(chan string, 2)
	hold := make(chan struct{})
	serverErrors := make(chan error, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection := connections.Add(1)
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			serverErrors <- err
			return
		}
		defer conn.CloseNow()
		_, control, err := streamRead(conn)
		if err != nil {
			serverErrors <- err
			return
		}
		controls <- string(control)
		if connection == 1 {
			_ = conn.Close(websocket.StatusInternalError, "reconnect")
			return
		}
		if err := streamWriteState(conn, &statepb.State{Timestamp: 2}); err != nil {
			serverErrors <- err
			return
		}
		<-hold
	}))
	defer server.Close()
	defer close(hold)

	statusStream := newTestStatusStreamWithOptions(t, server,
		publicstream.WithReconnectPolicy(publicstream.ReconnectPolicy{MaxAttempts: 3}),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := statusStream.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	for i := 0; i < 2; i++ {
		if control := receiveStreamValue(t, controls); control != `{"enable":true,"send":"all"}` {
			t.Fatalf("control %d = %q", i, control)
		}
	}
	message := receiveStreamValue(t, statusStream.Messages())
	if message.State.GetTimestamp() != 2 {
		t.Fatalf("reconnected timestamp = %d", message.State.GetTimestamp())
	}
	if got := connections.Load(); got != 2 {
		t.Fatalf("connections = %d, want 2", got)
	}
	if err := statusStream.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	assertNoServerError(t, serverErrors)
}

func TestStatusStreamRefreshesVersionAfterHandshake405(t *testing.T) {
	var versionCalls atomic.Int32
	var streamCalls atomic.Int32
	hold := make(chan struct{})
	serverErrors := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			call := versionCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]string{"api_semver": "24.4." + string(rune('0'+call))})
		case "/api/status/ws":
			streamCalls.Add(1)
			if r.URL.Query().Get("x-api-sem-ver") == "24.4.1" {
				http.Error(w, "Incompatible API version", http.StatusMethodNotAllowed)
				return
			}
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				serverErrors <- err
				return
			}
			defer conn.CloseNow()
			if _, _, err := streamRead(conn); err != nil {
				serverErrors <- err
				return
			}
			if err := streamWriteState(conn, &statepb.State{Timestamp: 2}); err != nil {
				serverErrors <- err
				return
			}
			<-hold
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	defer close(hold)

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithTimeout(time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	statusStream, err := client.NewStatusStream()
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := statusStream.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	receiveStreamValue(t, statusStream.Messages())
	if got := versionCalls.Load(); got != 2 {
		t.Fatalf("version calls = %d, want 2", got)
	}
	if got := streamCalls.Load(); got != 2 {
		t.Fatalf("stream calls = %d, want 2", got)
	}
	if err := statusStream.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	assertNoServerError(t, serverErrors)
}

func TestStatusStreamOmitsVersionQueryWhenNegotiationIsDisabled(t *testing.T) {
	versionQuery := make(chan string, 1)
	hold := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		versionQuery <- r.URL.Query().Get("x-api-sem-ver")
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		_, _, _ = streamRead(conn)
		<-hold
	}))
	defer server.Close()
	defer close(hold)

	statusStream := newTestStatusStreamWithOptionsAndClient(t, server, nil,
		WithVersionNegotiation(VersionNegotiationDisabled),
	)
	if err := statusStream.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got := receiveStreamValue(t, versionQuery); got != "" {
		t.Fatalf("x-api-sem-ver query = %q, want absent", got)
	}
	if err := statusStream.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestStatusStreamRejectsForbiddenHandshakeWithoutRetry(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "Forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	statusStream := newTestStatusStreamWithOptions(t, server,
		publicstream.WithReconnectPolicy(publicstream.ReconnectPolicy{MaxAttempts: 5}),
	)
	err := statusStream.Start(context.Background())
	var streamErr *publicstream.Error
	if !errors.As(err, &streamErr) || streamErr.StatusCode != http.StatusForbidden || !streamErr.Terminal {
		t.Fatalf("Start error = %T %v", err, err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("handshake calls = %d, want 1", got)
	}
	status := statusStream.Status()
	if status.Lifecycle != publicstream.LifecycleFailed || status.Access != publicstream.AccessRejected {
		t.Fatalf("status = %#v", status)
	}
}

func TestStatusStreamReconnectExhaustionIsTerminal(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer conn.CloseNow()
			if _, _, err := streamRead(conn); err != nil {
				return
			}
			_ = conn.Close(websocket.StatusInternalError, "retry")
			return
		}
		http.Error(w, "Unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	statusStream := newTestStatusStreamWithOptions(t, server,
		publicstream.WithReconnectPolicy(publicstream.ReconnectPolicy{MaxAttempts: 2}),
	)
	if err := statusStream.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	terminal := receiveStreamValue(t, statusStream.Errors())
	var streamErr *publicstream.Error
	if !errors.As(terminal, &streamErr) || !streamErr.Terminal || streamErr.StatusCode != http.StatusServiceUnavailable || streamErr.Attempt != 2 {
		t.Fatalf("terminal error = %#v", terminal)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("handshake calls = %d, want 3", got)
	}
}

func TestStatusStreamMalformedMessageRecoversAndHandlesPing(t *testing.T) {
	pingResult := make(chan error, 1)
	hold := make(chan struct{})
	serverErrors := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			serverErrors <- err
			return
		}
		defer conn.CloseNow()
		if _, _, err := streamRead(conn); err != nil {
			serverErrors <- err
			return
		}
		go func() {
			_, _, _ = conn.Read(context.Background())
		}()
		pingCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		pingResult <- conn.Ping(pingCtx)
		cancel()
		if err := conn.Write(context.Background(), websocket.MessageBinary, []byte{0xff}); err != nil {
			serverErrors <- err
			return
		}
		if err := streamWriteState(conn, &statepb.State{Timestamp: 9}); err != nil {
			serverErrors <- err
			return
		}
		<-hold
	}))
	defer server.Close()
	defer close(hold)

	statusStream := newTestStatusStream(t, server)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := statusStream.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := receiveStreamValue(t, pingResult); err != nil {
		t.Fatalf("server ping: %v", err)
	}
	malformed := receiveStreamValue(t, statusStream.Messages())
	if malformed.DecodeError == nil || len(malformed.Raw) != 1 {
		t.Fatalf("malformed message = %#v", malformed)
	}
	valid := receiveStreamValue(t, statusStream.Messages())
	if valid.DecodeError != nil || valid.State.GetTimestamp() != 9 {
		t.Fatalf("valid recovery message = %#v", valid)
	}
	if err := statusStream.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	assertNoServerError(t, serverErrors)
}

func TestStatusStreamRejectsOversizedMessageAndReconnects(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) != 1 {
			http.Error(w, "Unavailable", http.StatusServiceUnavailable)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		if _, _, err := streamRead(conn); err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
		defer cancel()
		if err := conn.Write(ctx, websocket.MessageBinary, bytes.Repeat([]byte{0}, (1<<20)+1)); err != nil {
			return
		}
		_, _, _ = conn.Read(ctx)
	}))
	defer server.Close()

	statusStream := newTestStatusStreamWithOptions(t, server,
		publicstream.WithReconnectPolicy(publicstream.ReconnectPolicy{MaxAttempts: 1}),
	)
	if err := statusStream.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	terminal := receiveStreamValue(t, statusStream.Errors())
	var streamErr *publicstream.Error
	if !errors.As(terminal, &streamErr) || !streamErr.Terminal || streamErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("terminal error = %#v", terminal)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("handshake calls = %d, want 2", got)
	}
	if message, ok := <-statusStream.Messages(); ok {
		t.Fatalf("oversized message was delivered: %#v", message)
	}
}

func TestStatusStreamFatalDeviceErrorIsDeliveredThenTerminates(t *testing.T) {
	var connections atomic.Int32
	serverErrors := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connections.Add(1)
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			serverErrors <- err
			return
		}
		defer conn.CloseNow()
		if _, _, err := streamRead(conn); err != nil {
			serverErrors <- err
			return
		}
		states := []*statepb.State{
			{Error: &errorpb.Error{Cause: errorpb.Cause_RESOURCE_LIMIT, Severity: errorpb.Severity_WARNING}},
			{Updates: []*statepb.StateUpdate{{State: &statepb.StateUpdate_DeviceName{DeviceName: &statepb.DeviceName{Name: "Desk"}}}}},
			{Error: &errorpb.Error{Cause: errorpb.Cause_RESOURCE_LIMIT, Severity: errorpb.Severity_FATAL}},
		}
		for _, state := range states {
			if err := streamWriteState(conn, state); err != nil {
				serverErrors <- err
				return
			}
		}
	}))
	defer server.Close()

	statusStream := newTestStatusStreamWithOptions(t, server,
		publicstream.WithReconnectPolicy(publicstream.ReconnectPolicy{MaxAttempts: 3}),
	)
	if err := statusStream.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	warning := receiveStreamValue(t, statusStream.Messages())
	if warning.DeviceError == nil || warning.DeviceError.Severity != errorpb.Severity_WARNING {
		t.Fatalf("warning message = %#v", warning)
	}
	receiveStreamValue(t, statusStream.Messages())
	fatal := receiveStreamValue(t, statusStream.Messages())
	if fatal.DeviceError == nil || fatal.DeviceError.Severity != errorpb.Severity_FATAL {
		t.Fatalf("fatal message = %#v", fatal)
	}
	terminal := receiveStreamValue(t, statusStream.Errors())
	var streamErr *publicstream.Error
	if !errors.As(terminal, &streamErr) || !streamErr.Terminal || streamErr.Operation != "device error" {
		t.Fatalf("terminal error = %T %v", terminal, terminal)
	}
	if got := connections.Load(); got != 1 {
		t.Fatalf("connections = %d, want 1", got)
	}
	if status := statusStream.Status(); status.Lifecycle != publicstream.LifecycleFailed {
		t.Fatalf("status = %#v", status)
	}
	assertNoServerError(t, serverErrors)
}

func TestStatusStreamSlowConsumerFailsWithoutDroppingSilently(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		if _, _, err := streamRead(conn); err != nil {
			return
		}
		for i := 0; i < 80; i++ {
			if err := streamWriteState(conn, &statepb.State{Timestamp: uint64(i)}); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	statusStream := newTestStatusStream(t, server)
	if err := statusStream.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	terminal := receiveStreamValue(t, statusStream.Errors())
	var streamErr *publicstream.Error
	if !errors.As(terminal, &streamErr) || streamErr.Operation != "deliver" || !streamErr.Terminal {
		t.Fatalf("terminal error = %T %v", terminal, terminal)
	}
}

func TestStatusStreamStopIsIdempotentAndRemoteIsRejected(t *testing.T) {
	hold := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		_, _, _ = streamRead(conn)
		<-hold
	}))
	defer server.Close()
	defer close(hold)

	statusStream := newTestStatusStream(t, server)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := statusStream.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := statusStream.Stop(); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	if err := statusStream.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
	if status := statusStream.Status(); status.Lifecycle != publicstream.LifecycleStopped {
		t.Fatalf("status = %#v", status)
	}
	if _, ok := <-statusStream.Messages(); ok {
		t.Fatal("Messages channel remained open")
	}

	remote, err := NewClient(
		WithEndpointMode(EndpointRemote),
		WithBaseURL("http://busybar.remote.invalid"),
		WithHTTPClient(&http.Client{}),
	)
	if err != nil {
		t.Fatalf("remote NewClient: %v", err)
	}
	_, err = remote.NewStatusStream()
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("remote stream error = %T %v", err, err)
	}
}

func newTestStatusStream(t *testing.T, server *httptest.Server, clientOptions ...Option) publicstream.Stream {
	t.Helper()
	return newTestStatusStreamWithOptionsAndClient(t, server, nil, clientOptions...)
}

func newTestStatusStreamWithOptions(t *testing.T, server *httptest.Server, options ...publicstream.Option) publicstream.Stream {
	t.Helper()
	return newTestStatusStreamWithOptionsAndClient(t, server, options)
}

func newTestStatusStreamWithOptionsAndClient(
	t *testing.T,
	server *httptest.Server,
	streamOptions []publicstream.Option,
	clientOptions ...Option,
) publicstream.Stream {
	t.Helper()
	options := []Option{
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithTimeout(time.Second),
	}
	options = append(options, clientOptions...)
	client, err := NewClient(options...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.setCachedAPISemVerForTest("25.0.0")
	statusStream, err := client.NewStatusStream(streamOptions...)
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	return statusStream
}

func streamRead(conn *websocket.Conn) (websocket.MessageType, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()
	return conn.Read(ctx)
}

func streamWriteState(conn *websocket.Conn, state *statepb.State) error {
	payload, err := proto.Marshal(state)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()
	return conn.Write(ctx, websocket.MessageBinary, payload)
}

func receiveStreamValue[T any](t *testing.T, channel <-chan T) T {
	t.Helper()
	select {
	case value, ok := <-channel:
		if !ok {
			t.Fatal("stream channel closed before a value arrived")
		}
		return value
	case <-time.After(streamTestTimeout):
		t.Fatal("timed out waiting for stream value")
		var zero T
		return zero
	}
}

func waitForStreamStatus(t *testing.T, statusStream publicstream.Stream, match func(publicstream.Status) bool) publicstream.Status {
	t.Helper()
	if status := statusStream.Status(); match(status) {
		return status
	}
	deadline := time.After(streamTestTimeout)
	for {
		select {
		case status, ok := <-statusStream.Statuses():
			if !ok {
				t.Fatal("status channel closed before expected status")
			}
			if match(status) {
				return status
			}
		case <-deadline:
			t.Fatalf("timed out waiting for stream status; current = %#v", statusStream.Status())
		}
	}
}

func assertNoServerError(t *testing.T, errors <-chan error) {
	t.Helper()
	select {
	case err := <-errors:
		t.Fatalf("stream test server: %v", err)
	default:
	}
}
