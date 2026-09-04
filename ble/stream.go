package ble

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/lxdb/busylib-go/internal/statusdecode"
	"github.com/lxdb/busylib-go/internal/streamstate"
	publicstream "github.com/lxdb/busylib-go/stream"
)

const (
	statusPath          = "FFE1"
	statusMessageBuffer = 64
)

var errStatusConsumerTooSlow = errors.New("BLE status stream consumer is too slow")

type statusStream struct {
	client  *Client
	options publicstream.Options
	release func(*statusStream)
	state   *streamstate.State

	packets     chan []byte
	disconnects chan error
	fatal       chan error

	mu     sync.Mutex
	cancel context.CancelFunc
}

func newStatusStream(client *Client, options []publicstream.Option, release func(*statusStream)) (*statusStream, error) {
	resolved, err := publicstream.ResolveOptions(options...)
	if err != nil {
		return nil, err
	}
	return &statusStream{
		client:      client,
		options:     resolved,
		release:     release,
		state:       streamstate.New(statusMessageBuffer),
		packets:     make(chan []byte, statusMessageBuffer),
		disconnects: make(chan error, 1),
		fatal:       make(chan error, 1),
	}, nil
}

func (s *statusStream) Messages() <-chan publicstream.Message { return s.state.Messages() }
func (s *statusStream) Statuses() <-chan publicstream.Status  { return s.state.Statuses() }
func (s *statusStream) Status() publicstream.Status           { return s.state.Status() }
func (s *statusStream) Wait() error                           { return s.state.Wait() }

func (s *statusStream) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("BLE status stream context must not be nil")
	}
	s.mu.Lock()
	if err := s.state.Begin(); err != nil {
		s.mu.Unlock()
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()
	s.setConnecting(publicstream.LifecycleConnecting, 1, nil)
	if err := s.client.connection.EnableStateNotifications(runCtx, s.handlePacket); err != nil {
		s.finish(err)
		return err
	}
	s.setConnected(1)
	go s.run(runCtx)
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

func (s *statusStream) notifyDisconnect(err error) {
	select {
	case s.disconnects <- err:
	default:
	}
}

func (s *statusStream) handlePacket(packet []byte) {
	copy := bytes.Clone(packet)
	select {
	case s.packets <- copy:
	default:
		s.signalFatal(errStatusConsumerTooSlow)
	}
}

func (s *statusStream) signalFatal(err error) {
	select {
	case s.fatal <- err:
	default:
	}
}

func (s *statusStream) run(ctx context.Context) {
	assembler := newStateAssembler(int(s.client.config.maxMessageBytes))
	staleInterval := s.options.StaleAfter / 4
	if staleInterval < time.Millisecond {
		staleInterval = time.Millisecond
	}
	if staleInterval > 250*time.Millisecond {
		staleInterval = 250 * time.Millisecond
	}
	stale := time.NewTicker(staleInterval)
	defer stale.Stop()
	for {
		select {
		case <-ctx.Done():
			s.finish(nil)
			return
		case err := <-s.fatal:
			s.finish(&publicstream.Error{Operation: "receive", Path: statusPath, Terminal: true, Err: err})
			return
		case packet := <-s.packets:
			payload, complete, err := assembler.Push(packet)
			if err != nil {
				s.finish(&publicstream.Error{Operation: "assemble", Path: statusPath, Terminal: true, Err: err})
				return
			}
			if !complete {
				continue
			}
			message, fatal := statusdecode.DecodeBinary(payload, statusPath)
			if message.State != nil && message.DecodeError == nil {
				s.state.SetStatus(func(status *publicstream.Status) {
					status.Data = publicstream.DataFresh
					status.LastStateAt = message.ReceivedAt
				})
			}
			if err := s.state.Deliver(ctx, message, errStatusConsumerTooSlow); err != nil {
				s.finish(&publicstream.Error{Operation: "deliver", Path: statusPath, Terminal: true, Err: err})
				return
			}
			if fatal {
				s.finish(&publicstream.Error{Operation: "device error", Path: statusPath, Terminal: true, Err: message.DeviceError})
				return
			}
		case err := <-s.disconnects:
			if reconnectErr := s.reconnect(ctx, err); reconnectErr != nil {
				if ctx.Err() != nil {
					s.finish(nil)
				} else {
					s.finish(reconnectErr)
				}
				return
			}
			assembler.reset()
		case now := <-stale.C:
			status := s.state.Status()
			if status.Lifecycle != publicstream.LifecycleConnected || status.Data == publicstream.DataStale {
				continue
			}
			reference := status.LastStateAt
			if reference.IsZero() {
				reference = status.ConnectedAt
			}
			if !reference.IsZero() && now.Sub(reference) >= s.options.StaleAfter {
				s.state.SetStatus(func(status *publicstream.Status) { status.Data = publicstream.DataStale })
			}
		}
	}
}

func (s *statusStream) reconnect(ctx context.Context, cause error) error {
	var lastErr error
	for attempt := 1; attempt <= s.options.Reconnect.MaxAttempts; attempt++ {
		s.setConnecting(publicstream.LifecycleReconnecting, attempt, cause)
		if attempt > 1 && s.options.Reconnect.Delay > 0 {
			timer := time.NewTimer(s.options.Reconnect.Delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		lastErr = s.client.connection.Reconnect(ctx, s.client.config.connectTimeout)
		if lastErr == nil {
			s.client.transport.handleReconnect()
			lastErr = s.client.connection.EnableStateNotifications(ctx, s.handlePacket)
		}
		if lastErr == nil {
			s.setConnected(attempt)
			return nil
		}
		cause = lastErr
	}
	return &publicstream.Error{Operation: "reconnect", Path: statusPath, Attempt: s.options.Reconnect.MaxAttempts, Err: lastErr}
}

func (s *statusStream) setConnecting(lifecycle publicstream.Lifecycle, attempt int, err error) {
	s.state.SetStatus(func(status *publicstream.Status) {
		status.Lifecycle = lifecycle
		status.Access = publicstream.AccessUnknown
		status.Attempt = attempt
		status.LastError = err
		if status.LastStateAt.IsZero() {
			status.Data = publicstream.DataWaiting
		} else if lifecycle == publicstream.LifecycleReconnecting {
			status.Data = publicstream.DataStale
		}
	})
}

func (s *statusStream) setConnected(attempt int) {
	s.state.SetStatus(func(status *publicstream.Status) {
		status.Lifecycle = publicstream.LifecycleConnected
		status.Access = publicstream.AccessUnknown
		status.Attempt = attempt
		status.ConnectedAt = time.Now()
		status.LastError = nil
		if status.LastStateAt.IsZero() {
			status.Data = publicstream.DataWaiting
		}
	})
}

func (s *statusStream) finish(terminal error) {
	s.state.Finish(terminal, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), s.client.config.connectTimeout)
		defer cancel()
		err := s.client.connection.DisableStateNotifications(ctx)
		if s.release != nil {
			s.release(s)
		}
		return err
	})
}
