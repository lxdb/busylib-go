package remote

import (
	"errors"
	"fmt"
)

// ErrClosed reports use of a closed remote client or transport adapter.
var ErrClosed = errors.New("remote client is closed")

// ErrMessageTooLarge reports an MQTT payload that exceeds the configured limit.
var ErrMessageTooLarge = errors.New("remote message exceeds the configured limit")

// Error describes a remote MQTT transport failure.
type Error struct {
	Operation string
	Route     string
	Attempt   int
	Terminal  bool
	Err       error
}

// Error describes the failed operation without including MQTT payload data.
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("remote %s %s failed: %v", e.Operation, e.Route, e.Err)
	}
	return fmt.Sprintf("remote %s %s failed", e.Operation, e.Route)
}

// Unwrap returns the underlying transport, protocol, or lifecycle error.
func (e *Error) Unwrap() error { return e.Err }
