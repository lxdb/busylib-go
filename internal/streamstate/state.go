// Package streamstate owns the lifecycle state shared by status stream
// transports.
package streamstate

import (
	"context"
	"errors"
	"sync"

	publicstream "github.com/lxdb/busylib-go/stream"
)

// State owns status publication, message delivery, and stable completion.
type State struct {
	messages chan publicstream.Message
	statuses chan publicstream.Status
	done     chan struct{}

	mu       sync.Mutex
	status   publicstream.Status
	started  bool
	finished bool
	finalErr error
	finish   sync.Once
}

// New creates an idle stream state with a bounded message queue.
func New(messageBuffer int) *State {
	state := &State{
		messages: make(chan publicstream.Message, messageBuffer),
		statuses: make(chan publicstream.Status, 1),
		done:     make(chan struct{}),
		status: publicstream.Status{
			Lifecycle: publicstream.LifecycleIdle,
			Access:    publicstream.AccessUnknown,
			Data:      publicstream.DataWaiting,
		},
	}
	state.statuses <- state.status
	return state
}

// Begin marks the one-shot stream as started.
func (s *State) Begin() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return publicstream.ErrAlreadyStarted
	}
	s.started = true
	return nil
}

// StopBeforeStart atomically claims an idle stream for clean completion.
func (s *State) StopBeforeStart() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return false
	}
	s.started = true
	return true
}

// Messages returns received status-stream messages.
func (s *State) Messages() <-chan publicstream.Message { return s.messages }

// Statuses returns coalesced lifecycle snapshots.
func (s *State) Statuses() <-chan publicstream.Status { return s.statuses }

// Status returns the latest lifecycle snapshot.
func (s *State) Status() publicstream.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// SetStatus applies and publishes a lifecycle change unless the stream ended.
func (s *State) SetStatus(change func(*publicstream.Status)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	change(&s.status)
	s.publishStatusLocked()
}

// Deliver queues one message without allowing a slow consumer to block the
// transport. The caller supplies the terminal slow-consumer error.
func (s *State) Deliver(ctx context.Context, message publicstream.Message, slowConsumer error) error {
	select {
	case s.messages <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return slowConsumer
	}
}

// Finish closes the stream once. Cleanup runs before completion becomes
// observable, and any cleanup failure is joined with the terminal error.
func (s *State) Finish(terminal error, cleanup func() error) {
	s.finish.Do(func() {
		if cleanup != nil {
			terminal = errors.Join(terminal, cleanup())
		}
		s.mu.Lock()
		if terminal == nil {
			s.status.Lifecycle = publicstream.LifecycleStopped
		} else {
			s.status.Lifecycle = publicstream.LifecycleFailed
			s.status.LastError = terminal
		}
		s.finalErr = terminal
		s.finished = true
		s.publishStatusLocked()
		close(s.messages)
		close(s.statuses)
		close(s.done)
		s.mu.Unlock()
	})
}

// Wait blocks for completion and returns the stable terminal or cleanup error.
func (s *State) Wait() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return publicstream.ErrNotStarted
	}
	done := s.done
	s.mu.Unlock()
	<-done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finalErr
}

func (s *State) publishStatusLocked() {
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
