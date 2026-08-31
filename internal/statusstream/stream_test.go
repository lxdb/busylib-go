package statusstream

import (
	"testing"

	"github.com/lxdb/busylib-go/internal/streamstate"
)

func TestStopAfterCompletionReturnsStableResult(t *testing.T) {
	state := streamstate.New(1)
	if err := state.Begin(); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	state.Finish(nil, nil)
	stream := &Stream{state: state}

	if err := stream.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := stream.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}
