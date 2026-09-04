package statusstream

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/lxdb/busylib-go/internal/statusdecode"
	"github.com/lxdb/busylib-go/internal/streamstate"
	publicstream "github.com/lxdb/busylib-go/stream"
)

const (
	statusStreamPath = "/api/status/ws"
	messageBuffer    = 64
	messageReadLimit = 1 << 20
)

var errConsumerTooSlow = errors.New("status stream consumer is too slow")

// Config contains the root-client dependencies required by the local stream.
type Config struct {
	BaseURL            *url.URL
	HTTPClient         *http.Client
	Timeout            time.Duration
	LocalAccessKey     string
	VersionNegotiation bool
	APISemVer          func(context.Context) (string, error)
	RefreshAPISemVer   func(context.Context) (string, error)
}

// Stream implements the public one-shot stream lifecycle over WebSocket.
type Stream struct {
	config  Config
	options publicstream.Options
	state   *streamstate.State

	mu      sync.Mutex
	cancel  context.CancelFunc
	conn    *websocket.Conn
	writeMu sync.Mutex
}

// New validates config and creates an idle local status stream.
func New(config Config, options ...publicstream.Option) (*Stream, error) {
	resolved, err := publicstream.ResolveOptions(options...)
	if err != nil {
		return nil, err
	}
	if config.BaseURL == nil {
		return nil, errors.New("status stream base URL must not be nil")
	}
	if config.HTTPClient == nil {
		return nil, errors.New("status stream HTTP client must not be nil")
	}
	if config.VersionNegotiation && (config.APISemVer == nil || config.RefreshAPISemVer == nil) {
		return nil, errors.New("status stream version callbacks must not be nil when negotiation is enabled")
	}

	instance := &Stream{
		config:  config,
		options: resolved,
		state:   streamstate.New(messageBuffer),
	}
	return instance, nil
}

// Messages returns the ordered message channel.
func (s *Stream) Messages() <-chan publicstream.Message { return s.state.Messages() }

// Statuses returns the coalescing lifecycle-status channel.
func (s *Stream) Statuses() <-chan publicstream.Status { return s.state.Statuses() }

// Status returns the latest lifecycle status.
func (s *Stream) Status() publicstream.Status { return s.state.Status() }

// Wait returns the stream's stable completion result.
func (s *Stream) Wait() error { return s.state.Wait() }

// Start connects the one-shot stream and starts its receive loop.
func (s *Stream) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("status stream context must not be nil")
	}

	s.mu.Lock()
	if err := s.state.Begin(); err != nil {
		s.mu.Unlock()
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()

	s.setStatus(func(status *publicstream.Status) {
		status.Lifecycle = publicstream.LifecycleConnecting
		status.Access = publicstream.AccessUnknown
		status.Data = publicstream.DataWaiting
		status.LastError = nil
	})

	conn, err := s.connectWithRetry(runCtx, publicstream.LifecycleConnecting)
	if err != nil {
		if runCtx.Err() != nil {
			err = runCtx.Err()
			s.finish(err)
			return err
		}
		s.finish(err)
		return err
	}

	go s.run(runCtx, conn)
	return nil
}

// Stop requests shutdown and returns the stable completion result.
func (s *Stream) Stop() error {
	s.mu.Lock()
	if s.state.StopBeforeStart() {
		s.mu.Unlock()
		s.finish(nil)
		return s.Wait()
	}
	cancel := s.cancel
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return s.Wait()
}

// RequestSnapshot asks a connected stream to send all current state.
func (s *Stream) RequestSnapshot(ctx context.Context) error {
	if ctx == nil {
		return errors.New("status stream context must not be nil")
	}

	s.mu.Lock()
	conn := s.conn
	connected := s.state.Status().Lifecycle == publicstream.LifecycleConnected && conn != nil
	s.mu.Unlock()
	if !connected {
		return errors.New("status stream is not connected")
	}

	if err := s.writeText(ctx, conn, []byte(`{"send":"all"}`)); err != nil {
		_ = conn.CloseNow()
		return &publicstream.Error{
			Operation: "request snapshot",
			Path:      statusStreamPath,
			Err:       err,
		}
	}
	return nil
}

func (s *Stream) run(ctx context.Context, conn *websocket.Conn) {
	for {
		result := s.readConnection(ctx, conn)
		_ = conn.CloseNow()
		if ctx.Err() != nil {
			s.finish(nil)
			return
		}
		if result.terminal {
			s.finish(result.err)
			return
		}

		s.setStatus(func(status *publicstream.Status) {
			status.Lifecycle = publicstream.LifecycleReconnecting
			status.Access = publicstream.AccessUnknown
			if status.LastStateAt.IsZero() {
				status.Data = publicstream.DataWaiting
			} else {
				status.Data = publicstream.DataStale
			}
			status.LastError = result.err
		})

		next, err := s.connectWithRetry(ctx, publicstream.LifecycleReconnecting)
		if err != nil {
			if ctx.Err() != nil {
				s.finish(nil)
			} else {
				s.finish(err)
			}
			return
		}
		conn = next
	}
}

type readResult struct {
	err      error
	terminal bool
}

func (s *Stream) readConnection(ctx context.Context, conn *websocket.Conn) readResult {
	staleCtx, cancelStale := context.WithCancel(ctx)
	staleDone := make(chan struct{})
	go func() {
		defer close(staleDone)
		s.watchStaleness(staleCtx)
	}()
	defer func() {
		cancelStale()
		<-staleDone
	}()

	for {
		messageType, payload, err := conn.Read(ctx)
		if err != nil {
			return readResult{err: s.websocketError("read", err)}
		}

		message, fatal := decodeMessage(messageType, payload)
		if message.State != nil && message.DecodeError == nil {
			s.setStatus(func(status *publicstream.Status) {
				status.Data = publicstream.DataFresh
				status.LastStateAt = message.ReceivedAt
			})
		}
		if err := s.state.Deliver(ctx, message, errConsumerTooSlow); err != nil {
			if ctx.Err() != nil {
				return readResult{err: ctx.Err()}
			}
			return readResult{
				err: &publicstream.Error{
					Operation: "deliver",
					Path:      statusStreamPath,
					Terminal:  true,
					Err:       err,
				},
				terminal: true,
			}
		}
		if fatal {
			return readResult{
				err: &publicstream.Error{
					Operation: "device error",
					Path:      statusStreamPath,
					Terminal:  true,
					Err:       message.DeviceError,
				},
				terminal: true,
			}
		}
	}
}

func decodeMessage(messageType websocket.MessageType, payload []byte) (publicstream.Message, bool) {
	if messageType == websocket.MessageText {
		return publicstream.Message{
			Kind:       publicstream.MessageText,
			ReceivedAt: time.Now(),
			Raw:        append([]byte(nil), payload...),
			Text:       string(payload),
		}, false
	}
	return statusdecode.DecodeBinary(payload, statusStreamPath)
}

func (s *Stream) watchStaleness(ctx context.Context) {
	interval := s.options.StaleAfter / 4
	if interval < time.Millisecond {
		interval = time.Millisecond
	}
	if interval > 250*time.Millisecond {
		interval = 250 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.setStatus(func(status *publicstream.Status) {
				if status.Lifecycle != publicstream.LifecycleConnected || status.Data == publicstream.DataStale {
					return
				}
				reference := status.LastStateAt
				if reference.IsZero() {
					reference = status.ConnectedAt
				}
				if !reference.IsZero() && now.Sub(reference) >= s.options.StaleAfter {
					status.Data = publicstream.DataStale
				}
			})
		}
	}
}

func (s *Stream) connectWithRetry(ctx context.Context, lifecycle publicstream.Lifecycle) (*websocket.Conn, error) {
	var lastErr *publicstream.Error
	for attempt := 1; attempt <= s.options.Reconnect.MaxAttempts; attempt++ {
		s.setStatus(func(status *publicstream.Status) {
			status.Lifecycle = lifecycle
			status.Attempt = attempt
			status.Access = publicstream.AccessUnknown
		})

		conn, failure, retry, rejected := s.connectOnce(ctx, attempt)
		if failure == nil {
			s.mu.Lock()
			s.conn = conn
			s.mu.Unlock()
			s.setStatus(func(status *publicstream.Status) {
				status.Lifecycle = publicstream.LifecycleConnected
				status.Access = publicstream.AccessAccepted
				if status.LastStateAt.IsZero() {
					status.Data = publicstream.DataWaiting
				} else {
					status.Data = publicstream.DataStale
				}
				status.ConnectedAt = time.Now()
				status.LastError = nil
			})
			return conn, nil
		}
		lastErr = failure
		terminal := !retry || attempt == s.options.Reconnect.MaxAttempts
		if terminal {
			failure.Terminal = true
		}
		s.setStatus(func(status *publicstream.Status) {
			status.LastError = failure
			if rejected {
				status.Access = publicstream.AccessRejected
			}
		})
		if ctx.Err() != nil {
			return nil, failure
		}
		if terminal {
			return nil, failure
		}
		if err := wait(ctx, s.options.Reconnect.Delay); err != nil {
			return nil, &publicstream.Error{
				Operation: "reconnect",
				Path:      statusStreamPath,
				Attempt:   attempt,
				Terminal:  true,
				Err:       err,
			}
		}
	}
	return nil, lastErr
}

func (s *Stream) connectOnce(ctx context.Context, attempt int) (*websocket.Conn, *publicstream.Error, bool, bool) {
	compatibilityRetried := false
	for {
		target := *s.config.BaseURL
		target.Path = statusStreamPath
		query := target.Query()
		if s.config.LocalAccessKey != "" {
			query.Set("x-api-token", s.config.LocalAccessKey)
		}
		if s.config.VersionNegotiation {
			version, err := s.config.APISemVer(ctx)
			if err != nil {
				return nil, streamError("version", attempt, 0, err), true, false
			}
			query.Set("x-api-sem-ver", version)
		}
		target.RawQuery = query.Encode()

		dialCtx, cancel := s.withTimeout(ctx)
		conn, response, err := websocket.Dial(dialCtx, target.String(), &websocket.DialOptions{
			HTTPClient:      s.config.HTTPClient,
			CompressionMode: websocket.CompressionDisabled,
		})
		cancel()
		if err != nil {
			statusCode := 0
			if response != nil {
				statusCode = response.StatusCode
				_ = response.Body.Close()
			}
			if statusCode == http.StatusMethodNotAllowed && s.config.VersionNegotiation && !compatibilityRetried {
				compatibilityRetried = true
				if _, refreshErr := s.config.RefreshAPISemVer(ctx); refreshErr != nil {
					return nil, streamError("version refresh", attempt, statusCode, refreshErr), true, false
				}
				continue
			}
			failure := streamError("connect", attempt, statusCode, err)
			rejected := statusCode == http.StatusForbidden
			retry := statusCode == 0 || statusCode >= 500
			return nil, failure, retry, rejected
		}
		conn.SetReadLimit(messageReadLimit)
		if err := s.writeText(ctx, conn, []byte(`{"enable":true,"send":"all"}`)); err != nil {
			_ = conn.CloseNow()
			return nil, streamError("enable", attempt, 0, err), true, false
		}
		return conn, nil, false, false
	}
}

func (s *Stream) writeText(ctx context.Context, conn *websocket.Conn, payload []byte) error {
	writeCtx, cancel := s.withTimeout(ctx)
	defer cancel()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return conn.Write(writeCtx, websocket.MessageText, payload)
}

func (s *Stream) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.config.Timeout > 0 {
		return context.WithTimeout(ctx, s.config.Timeout)
	}
	return context.WithCancel(ctx)
}

func (s *Stream) websocketError(operation string, err error) *publicstream.Error {
	closeCode := int(websocket.CloseStatus(err))
	if closeCode < 0 {
		closeCode = 0
	}
	return &publicstream.Error{
		Operation: operation,
		Path:      statusStreamPath,
		CloseCode: closeCode,
		Err:       err,
	}
}

func streamError(operation string, attempt, statusCode int, err error) *publicstream.Error {
	return &publicstream.Error{
		Operation:  operation,
		Path:       statusStreamPath,
		Attempt:    attempt,
		StatusCode: statusCode,
		Err:        err,
	}
}

func (s *Stream) setStatus(change func(*publicstream.Status)) {
	s.state.SetStatus(change)
}

func (s *Stream) finish(err error) {
	s.mu.Lock()
	s.conn = nil
	s.mu.Unlock()
	s.state.Finish(err, nil)
}

func wait(ctx context.Context, duration time.Duration) error {
	if duration == 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
