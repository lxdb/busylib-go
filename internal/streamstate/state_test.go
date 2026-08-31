package streamstate

import (
	"context"
	"errors"
	"testing"

	publicstream "github.com/lxdb/busylib-go/stream"
)

func TestStateCompletionContract(t *testing.T) {
	state := New(1)
	if err := state.Wait(); !errors.Is(err, publicstream.ErrNotStarted) {
		t.Fatalf("Wait before Begin = %v, want ErrNotStarted", err)
	}
	if err := state.Begin(); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := state.Begin(); !errors.Is(err, publicstream.ErrAlreadyStarted) {
		t.Fatalf("second Begin = %v, want ErrAlreadyStarted", err)
	}
	message := publicstream.Message{Kind: publicstream.MessageText, Text: "ready"}
	if err := state.Deliver(context.Background(), message, errors.New("slow")); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if got := <-state.Messages(); got.Text != "ready" {
		t.Fatalf("message = %#v", got)
	}

	terminalErr := errors.New("terminal")
	cleanupErr := errors.New("cleanup")
	state.Finish(terminalErr, func() error { return cleanupErr })
	for range 2 {
		err := state.Wait()
		if !errors.Is(err, terminalErr) || !errors.Is(err, cleanupErr) {
			t.Fatalf("Wait = %v, want joined terminal and cleanup errors", err)
		}
	}
	if _, ok := <-state.Messages(); ok {
		t.Fatal("Messages remained open")
	}
	var final publicstream.Status
	for status := range state.Statuses() {
		final = status
	}
	if final.Lifecycle != publicstream.LifecycleFailed || !errors.Is(final.LastError, terminalErr) || !errors.Is(final.LastError, cleanupErr) {
		t.Fatalf("final status = %#v", final)
	}
}

func TestStateStopBeforeStartCompletesCleanly(t *testing.T) {
	state := New(1)
	if !state.StopBeforeStart() {
		t.Fatal("StopBeforeStart did not claim idle state")
	}
	if state.StopBeforeStart() {
		t.Fatal("second StopBeforeStart claimed started state")
	}
	state.Finish(nil, nil)
	if err := state.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if status := state.Status(); status.Lifecycle != publicstream.LifecycleStopped {
		t.Fatalf("status = %#v", status)
	}
}
