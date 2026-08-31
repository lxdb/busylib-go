package remote

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/lxdb/busylib-go/internal/statusdecode"
	"github.com/lxdb/busylib-go/internal/streamstate"
	publicstream "github.com/lxdb/busylib-go/stream"
)

const remoteMessageBuffer = 64

var errRemoteConsumerTooSlow = errors.New("remote status stream consumer is too slow")

type statusStream struct {
	transport       Transport
	requestTopic    string
	responseTopic   string
	lease           time.Duration
	startPayload    []byte
	timeout         time.Duration
	maxMessageBytes int64
	options         publicstream.Options
	release         func(*statusStream)

	state *streamstate.State

	mu           sync.Mutex
	activated    bool
	cancel       context.CancelFunc
	subscription Subscription
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
		transport:       transport,
		requestTopic:    "sessions/" + config.sessionID + "/down/v1/stream-request",
		responseTopic:   "sessions/" + config.sessionID + "/up/v1/stream-response/" + config.clientID,
		lease:           config.streamLease,
		startPayload:    payload,
		timeout:         config.requestTimeout,
		maxMessageBytes: config.maxMessageBytes,
		options:         resolved,
		release:         release,
		state:           streamstate.New(remoteMessageBuffer),
	}
	return stream, nil
}

func (s *statusStream) Messages() <-chan publicstream.Message { return s.state.Messages() }
func (s *statusStream) Statuses() <-chan publicstream.Status  { return s.state.Statuses() }
func (s *statusStream) Status() publicstream.Status           { return s.state.Status() }
func (s *statusStream) Wait() error                           { return s.state.Wait() }

func (s *statusStream) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("remote status stream context must not be nil")
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
	subscription, err := s.connectWithRetry(runCtx, publicstream.LifecycleConnecting)
	if err != nil {
		if runCtx.Err() != nil {
			err = runCtx.Err()
			s.finish(err)
			return err
		}
		s.finish(err)
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
			if int64(len(result.message.Payload)) > s.maxMessageBytes {
				return remoteReadResult{
					err: &publicstream.Error{
						Operation: "receive",
						Path:      s.responseTopic,
						Terminal:  true,
						Err:       ErrMessageTooLarge,
					},
					terminal: true,
				}
			}
			message, fatal := statusdecode.DecodeBinary(result.message.Payload, s.responseTopic)
			if message.State != nil && message.DecodeError == nil {
				s.setStatus(func(status *publicstream.Status) {
					status.Data = publicstream.DataFresh
					status.LastStateAt = message.ReceivedAt
				})
			}
			if err := s.state.Deliver(ctx, message, errRemoteConsumerTooSlow); err != nil {
				if ctx.Err() != nil {
					return remoteReadResult{err: ctx.Err()}
				}
				return remoteReadResult{
					err:      &publicstream.Error{Operation: "deliver", Path: s.responseTopic, Terminal: true, Err: err},
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
	subscription, err := s.transport.Subscribe(callCtx, SubscriptionRequest{
		Topic:           s.responseTopic,
		QoS:             QoSAtMostOnce,
		MaxPayloadBytes: s.maxMessageBytes,
	})
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
	s.state.SetStatus(change)
}

func (s *statusStream) finish(err error) {
	s.state.Finish(err, func() error {
		s.publishStop()
		s.mu.Lock()
		s.subscription = nil
		cleanupErr := errors.Join(s.stopErr, s.closeErr)
		s.mu.Unlock()
		if s.release != nil {
			s.release(s)
		}
		return cleanupErr
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
