package pahotransport

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/eclipse/paho.golang/paho"
	"github.com/lxdb/busylib-go/remote"
)

type fakeUnsubscriber struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeUnsubscriber) Unsubscribe(context.Context, *paho.Unsubscribe) (*paho.Unsuback, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return &paho.Unsuback{}, nil
}

func TestSubscriptionCloseStopsDeliveryAndUnblocksReceive(t *testing.T) {
	manager := &fakeUnsubscriber{}
	subscription := &subscription{
		manager:  manager,
		topic:    "status",
		messages: make(chan remote.Message, 1),
		done:     make(chan struct{}),
	}
	received := make(chan error, 1)
	go func() {
		_, err := subscription.Receive(context.Background())
		received <- err
	}()

	if err := subscription.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := subscription.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if err := <-received; !errors.Is(err, remote.ErrClosed) {
		t.Fatalf("Receive error = %v, want remote.ErrClosed", err)
	}
	if handled, err := subscription.deliver(remote.Message{Topic: "status"}); !handled || err != nil {
		t.Fatalf("delivery after Close = handled %t, error %v", handled, err)
	}
	manager.mu.Lock()
	calls := manager.calls
	manager.mu.Unlock()
	if calls != 1 {
		t.Fatalf("unsubscribe calls = %d, want 1", calls)
	}
}

func TestSubscriptionDeliveryCanOverlapClose(t *testing.T) {
	subscription := &subscription{
		manager:  &fakeUnsubscriber{},
		topic:    "status",
		messages: make(chan remote.Message, 1),
		done:     make(chan struct{}),
	}
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, _ = subscription.deliver(remote.Message{Topic: "status"})
		}()
	}
	close(start)
	if err := subscription.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	workers.Wait()
}
