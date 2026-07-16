package stream

import "fmt"

// Error is a status-stream transport, lifecycle, or protocol failure. Path is
// intentionally query-free so access credentials cannot leak through errors.
type Error struct {
	Operation  string
	Path       string
	Attempt    int
	StatusCode int
	CloseCode  int
	Terminal   bool
	Err        error
}

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

func (e *Error) Unwrap() error { return e.Err }

func (e *DeviceError) Error() string {
	return fmt.Sprintf("BUSY Bar status stream reported %s severity %s", e.Cause.String(), e.Severity.String())
}
