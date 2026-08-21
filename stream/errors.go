package stream

import (
	"errors"
	"fmt"
)

// ErrSnapshotUnsupported reports that a transport has no snapshot command.
// Remote MQTT streams have this behavior; use snapshot.Collect with the
// remote device client when a point-in-time snapshot is required.
var ErrSnapshotUnsupported = errors.New("status stream snapshot requests are unsupported by this transport")

// Error is a status-stream transport, lifecycle, or protocol failure. Path is
// a query-free WebSocket path or an MQTT topic, so credentials cannot leak
// through errors.
type Error struct {
	Operation  string
	Path       string
	Attempt    int
	StatusCode int
	CloseCode  int
	Terminal   bool
	Err        error
}

// Error describes the failed stream operation without exposing URL queries.
func (e *Error) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("status stream %s %s failed: HTTP %d", e.Operation, e.Path, e.StatusCode)
	}
	if e.CloseCode != 0 {
		return fmt.Sprintf("status stream %s %s failed: WebSocket close %d: %v", e.Operation, e.Path, e.CloseCode, e.Err)
	}
	if e.Err != nil {
		return fmt.Sprintf("status stream %s %s failed: %v", e.Operation, e.Path, e.Err)
	}
	return fmt.Sprintf("status stream %s %s failed", e.Operation, e.Path)
}

// Unwrap returns the underlying transport, protocol, or lifecycle error.
func (e *Error) Unwrap() error { return e.Err }

// Error describes the cause and severity reported by the firmware.
func (e *DeviceError) Error() string {
	return fmt.Sprintf("BUSY Bar status stream reported %s severity %s", e.Cause.String(), e.Severity.String())
}
