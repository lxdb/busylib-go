package remote

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/lxdb/busylib-go/internal/statusdecode"
	publicstream "github.com/lxdb/busylib-go/stream"
)

const remoteMessageBuffer = 64

var errRemoteConsumerTooSlow = errors.New("remote status stream consumer is too slow")

type statusStream struct {
	transport     Transport
	requestTopic  string
	responseTopic string
	lease         time.Duration
	startPayload  []byte
	timeout       time.Duration
	options       publicstream.Options
	release       func(*statusStream)

	messages chan publicstream.Message
	statuses chan publicstream.Status
	errors   chan error
	done     chan struct{}

	mu           sync.Mutex
	status       publicstream.Status
	started      bool
	activated    bool
	cancel       context.CancelFunc
	subscription Subscription
	closeOnce    sync.Once
	stopOnce     sync.Once
	stopErr      error
	closeErr     error
}

func newStatusStream(transport Transport, config clientConfig, options []publicstream.Option, release func(*statusStream)) (*statusStream, error) {
	resolved, err := publicstream.ResolveOptions(options...)
	if err != nil {
		return nil, err
	}
	payload := []byte("{}")
	if config.streamLimit.MaxCount != 0 {
		value := struct {
			MessageLimits struct {
				MaxCount  uint32  `json:"max_count"`
				IntervalS float64 `json:"interval_s"`
			} `json:"message_limits"`
		}{}
		value.MessageLimits.MaxCount = config.streamLimit.MaxCount
		value.MessageLimits.IntervalS = config.streamLimit.Interval.Seconds()
		payload, err = json.Marshal(value)
		if err != nil {
			return nil, err
		}
	}
	stream := &statusStream{
		transport:     transport,
		requestTopic:  "sessions/" + config.sessionID + "/down/v1/stream-request",
		responseTopic: "sessions/" + config.sessionID + "/up/v1/stream-response/" + config.clientID,
		lease:         config.streamLease,
		startPayload:  payload,
		timeout:       config.requestTimeout,
		options:       resolved,
		release:       release,
		messages:      make(chan publicstream.Message, remoteMessageBuffer),
		statuses:      make(chan publicstream.Status, 1),
		errors:        make(chan error, 1),
		done:          make(chan struct{}),
		status: publicstream.Status{
			Lifecycle: publicstream.LifecycleIdle,
			Access:    publicstream.AccessUnknown,
			Data:      publicstream.DataWaiting,
		},
	}
	stream.statuses <- stream.status
	return stream, nil
}

func (s *statusStream) Messages() <-chan publicstream.Message { return s.messages }
func (s *statusStream) Statuses() <-chan publicstream.Status  { return s.statuses }
func (s *statusStream) Errors() <-chan error                  { return s.errors }

func (s *statusStream) Status() publicstream.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *statusStream) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("remote status stream context must not be nil")
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("remote status stream has already been started")
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
	subscription, err := s.connectWithRetry(runCtx, publicstream.LifecycleConnecting)
	if err != nil {
		if runCtx.Err() != nil {
			err = runCtx.Err()
			s.finish(nil, false)
			return err
		}
		s.finish(err, false)
		return err
	}
	s.mu.Lock()
	s.activated = true
	s.mu.Unlock()
	go s.run(runCtx, subscription)
	return nil
}

func (s *statusStream) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.started = true
		s.mu.Unlock()
		s.finish(nil, false)
		return nil
	}
	cancel := s.cancel
	subscription := s.subscription
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if subscription != nil {
		if err := subscription.Close(); err != nil {
			closeErr := streamTransportError("close", s.responseTopic, 0, err)
			markStreamErrorTerminal(closeErr)
			s.recordCloseError(closeErr)
		}
	}
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return errors.Join(s.stopErr, s.closeErr)
}

func (s *statusStream) RequestSnapshot(context.Context) error {
	return publicstream.ErrSnapshotUnsupported
}

func (s *statusStream) run(ctx context.Context, subscription Subscription) {
	for {
		result := s.readSubscription(ctx, subscription)
		if err := subscription.Close(); err != nil {
			closeErr := streamTransportError("close", s.responseTopic, 0, err)
			s.recordCloseError(closeErr)
			if ctx.Err() == nil {
				result.err = errors.Join(result.err, closeErr)
			}
		}
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
		subscription = next
	}
}

type remoteReadResult struct {
	err      error
	terminal bool
}

type subscriptionResult struct {
	message Message
	err     error
}

type guardedSubscription struct {
	Subscription
	once sync.Once
	err  error
}

func (s *guardedSubscription) Close() error {
	s.once.Do(func() { s.err = s.Subscription.Close() })
	return s.err
}

func (s *statusStream) readSubscription(ctx context.Context, subscription Subscription) remoteReadResult {
	renew := time.NewTimer(s.lease / 2)
	defer renew.Stop()
	staleInterval := s.options.StaleAfter / 4
	if staleInterval < time.Millisecond {
		staleInterval = time.Millisecond
	}
	if staleInterval > 250*time.Millisecond {
		staleInterval = 250 * time.Millisecond
	}
	stale := time.NewTicker(staleInterval)
	defer stale.Stop()
	received := make(chan subscriptionResult, 1)
	go func() {
		for {
			message, err := subscription.Receive(ctx)
			select {
			case received <- subscriptionResult{message: message, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return remoteReadResult{err: ctx.Err()}
		case <-renew.C:
			if err := s.publishStart(ctx); err != nil {
				return remoteReadResult{err: streamTransportError("renew", s.requestTopic, 0, err)}
			}
			renew.Reset(s.lease / 2)
		case now := <-stale.C:
			s.markStale(now)
		case result := <-received:
			if result.err != nil {
				return remoteReadResult{err: streamTransportError("receive", s.responseTopic, 0, result.err)}
			}
			if result.message.Topic != s.responseTopic {
				continue
			}
			message, fatal := statusdecode.DecodeBinary(result.message.Payload, s.responseTopic)
			if message.State != nil && message.DecodeError == nil {
				s.setStatus(func(status *publicstream.Status) {
					status.Data = publicstream.DataFresh
					status.LastStateAt = message.ReceivedAt
				})
			}
			select {
			case s.messages <- message:
			case <-ctx.Done():
				return remoteReadResult{err: ctx.Err()}
			default:
				return remoteReadResult{
					err:      &publicstream.Error{Operation: "deliver", Path: s.responseTopic, Terminal: true, Err: errRemoteConsumerTooSlow},
					terminal: true,
				}
			}
			if fatal {
				return remoteReadResult{
					err:      &publicstream.Error{Operation: "device error", Path: s.responseTopic, Terminal: true, Err: message.DeviceError},
					terminal: true,
				}
			}
		}
	}
}

func (s *statusStream) connectWithRetry(ctx context.Context, lifecycle publicstream.Lifecycle) (Subscription, error) {
	var lastErr *publicstream.Error
	for attempt := 1; attempt <= s.options.Reconnect.MaxAttempts; attempt++ {
		s.setStatus(func(status *publicstream.Status) {
			status.Lifecycle = lifecycle
			status.Attempt = attempt
			status.Access = publicstream.AccessUnknown
		})
		subscription, err := s.subscribe(ctx)
		if err == nil {
			err = s.publishStart(ctx)
		}
		if err == nil {
			s.mu.Lock()
			s.subscription = subscription
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
			return subscription, nil
		}
		if subscription != nil {
			if closeErr := subscription.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}
		lastErr = streamTransportError("connect", s.requestTopic, attempt, err)
		if attempt == s.options.Reconnect.MaxAttempts {
			markStreamErrorTerminal(lastErr)
		}
		s.setStatus(func(status *publicstream.Status) { status.LastError = lastErr })
		if ctx.Err() != nil || lastErr.Terminal {
			return nil, lastErr
		}
		if err := waitRemote(ctx, s.options.Reconnect.Delay); err != nil {
			return nil, streamTransportError("reconnect", s.requestTopic, attempt, err)
		}
	}
	return nil, lastErr
}

func (s *statusStream) subscribe(ctx context.Context) (Subscription, error) {
	callCtx, cancel := s.withTimeout(ctx)
	defer cancel()
	subscription, err := s.transport.Subscribe(callCtx, SubscriptionRequest{Topic: s.responseTopic, QoS: QoSAtMostOnce})
	if err != nil {
		return nil, err
	}
	if subscription == nil {
		return nil, errors.New("transport returned a nil subscription")
	}
	return &guardedSubscription{Subscription: subscription}, nil
}

func (s *statusStream) publishStart(ctx context.Context) error {
	expiry := uint32(s.lease / time.Second)
	callCtx, cancel := s.withTimeout(ctx)
	defer cancel()
	return s.transport.Publish(callCtx, Message{
		Topic:   s.requestTopic,
		Payload: append([]byte(nil), s.startPayload...),
		QoS:     QoSAtLeastOnce,
		Properties: Properties{
			ResponseTopic:                s.responseTopic,
			MessageExpiryIntervalSeconds: &expiry,
		},
	})
}

func (s *statusStream) publishStop() {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		activated := s.activated
		s.mu.Unlock()
		if !activated {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
		defer cancel()
		if err := s.transport.Publish(ctx, Message{Topic: s.requestTopic, QoS: QoSAtLeastOnce}); err != nil {
			s.mu.Lock()
			stopErr := streamTransportError("stop", s.requestTopic, 0, err)
			markStreamErrorTerminal(stopErr)
			s.stopErr = stopErr
			s.mu.Unlock()
		}
	})
}

func (s *statusStream) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.timeout > 0 {
		return context.WithTimeout(ctx, s.timeout)
	}
	return context.WithCancel(ctx)
}

func (s *statusStream) markStale(now time.Time) {
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

func (s *statusStream) recordCloseError(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	if s.closeErr == nil {
		s.closeErr = err
	}
	s.mu.Unlock()
}

func (s *statusStream) setStatus(change func(*publicstream.Status)) {
	s.mu.Lock()
	change(&s.status)
	s.publishStatusLocked()
	s.mu.Unlock()
}

func (s *statusStream) publishStatusLocked() {
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

func (s *statusStream) finish(err error, emitTerminal bool) {
	s.closeOnce.Do(func() {
		s.publishStop()
		s.mu.Lock()
		s.subscription = nil
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
		if s.release != nil {
			s.release(s)
		}
		close(s.done)
	})
}

func streamTransportError(operation, path string, attempt int, err error) *publicstream.Error {
	return &publicstream.Error{
		Operation: operation,
		Path:      path,
		Attempt:   attempt,
		Err: &Error{
			Operation: operation,
			Route:     path,
			Attempt:   attempt,
			Err:       err,
		},
	}
}

func markStreamErrorTerminal(err *publicstream.Error) {
	err.Terminal = true
	var remoteErr *Error
	if errors.As(err, &remoteErr) {
		remoteErr.Terminal = true
	}
}

func waitRemote(ctx context.Context, duration time.Duration) error {
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
