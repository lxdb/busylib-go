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
	publicstream "github.com/lxdb/busylib-go/stream"
)

const (
	statusStreamPath = "/api/status/ws"
	messageBuffer    = 64
	messageReadLimit = 1 << 20
)

var errConsumerTooSlow = errors.New("status stream consumer is too slow")

type Config struct {
	BaseURL            *url.URL
	HTTPClient         *http.Client
	Timeout            time.Duration
	LocalAccessKey     string
	VersionNegotiation bool
	APISemVer          func(context.Context) (string, error)
	RefreshAPISemVer   func(context.Context) (string, error)
}

type Stream struct {
	config  Config
	options publicstream.Options

	messages chan publicstream.Message
	statuses chan publicstream.Status
	errors   chan error
	done     chan struct{}

	mu        sync.Mutex
	status    publicstream.Status
	started   bool
	cancel    context.CancelFunc
	conn      *websocket.Conn
	writeMu   sync.Mutex
	closeOnce sync.Once
}

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
		config:   config,
		options:  resolved,
		messages: make(chan publicstream.Message, messageBuffer),
		statuses: make(chan publicstream.Status, 1),
		errors:   make(chan error, 1),
		done:     make(chan struct{}),
		status: publicstream.Status{
			Lifecycle: publicstream.LifecycleIdle,
			Access:    publicstream.AccessUnknown,
			Data:      publicstream.DataWaiting,
		},
	}
	instance.statuses <- instance.status
	return instance, nil
}

func (s *Stream) Messages() <-chan publicstream.Message { return s.messages }
func (s *Stream) Statuses() <-chan publicstream.Status  { return s.statuses }
func (s *Stream) Errors() <-chan error                  { return s.errors }

func (s *Stream) Status() publicstream.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Stream) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("status stream context must not be nil")
	}

	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("status stream has already been started")
	}
	s.started = true
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
			s.finish(nil, false)
			return err
		}
		s.finish(err, false)
		return err
	}

	go s.run(runCtx, conn)
	return nil
}

func (s *Stream) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.started = true
		s.mu.Unlock()
		s.finish(nil, false)
		return nil
	}
	cancel := s.cancel
	conn := s.conn
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	var closeErr error
	if conn != nil {
		closeErr = conn.CloseNow()
	}
	<-s.done
	return closeErr
}

func (s *Stream) RequestSnapshot(ctx context.Context) error {
	if ctx == nil {
		return errors.New("status stream context must not be nil")
	}

	s.mu.Lock()
	conn := s.conn
	connected := s.status.Lifecycle == publicstream.LifecycleConnected && conn != nil
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
			s.finish(nil, false)
			return
		}
		if result.terminal {
			s.finish(result.err, true)
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
				s.finish(nil, false)
			} else {
				s.finish(err, true)
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
		select {
		case s.messages <- message:
		case <-ctx.Done():
			return readResult{err: ctx.Err()}
		default:
			return readResult{
				err: &publicstream.Error{
					Operation: "deliver",
					Path:      statusStreamPath,
					Terminal:  true,
					Err:       errConsumerTooSlow,
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
			s.status.Lifecycle = publicstream.LifecycleConnected
			s.status.Access = publicstream.AccessAccepted
			if s.status.LastStateAt.IsZero() {
				s.status.Data = publicstream.DataWaiting
			} else {
				s.status.Data = publicstream.DataStale
			}
			s.status.ConnectedAt = time.Now()
			s.status.LastError = nil
			s.publishStatusLocked()
			s.mu.Unlock()
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
	s.mu.Lock()
	change(&s.status)
	s.publishStatusLocked()
	s.mu.Unlock()
}

func (s *Stream) publishStatusLocked() {
	select {
	case s.statuses <- s.status:
		return
	default:
	}
	select {
	case <-s.statuses:
	default:
	}
	select {
	case s.statuses <- s.status:
	default:
	}
}

func (s *Stream) finish(err error, emitTerminal bool) {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.conn = nil
		if err == nil {
			s.status.Lifecycle = publicstream.LifecycleStopped
		} else {
			s.status.Lifecycle = publicstream.LifecycleFailed
			s.status.LastError = err
		}
		s.publishStatusLocked()
		s.mu.Unlock()

		if emitTerminal && err != nil {
			s.errors <- err
		}
		close(s.messages)
		close(s.statuses)
		close(s.errors)
		close(s.done)
	})
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
