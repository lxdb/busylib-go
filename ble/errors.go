package ble

import (
	"errors"
	"fmt"
)

var (
	// ErrUnsupported reports that no BLE backend is available for the current
	// operating system or build configuration.
	ErrUnsupported = errors.New("BLE is unsupported on this platform")
	// ErrClosed reports use of a closed BLE client.
	ErrClosed = errors.New("BLE client is closed")
	// ErrNotFound reports that a scan found no BUSY Bar or that CoreBluetooth
	// could not retrieve the requested identifier.
	ErrNotFound = errors.New("BUSY Bar BLE peripheral was not found")
	// ErrDisconnected reports an operation attempted without an active link.
	ErrDisconnected = errors.New("BLE peripheral is disconnected")
	// ErrOutcomeUnknown reports that a request began writing but no complete
	// response proved whether the device applied it. The affected HTTP session
	// cannot safely process another request.
	ErrOutcomeUnknown = errors.New("BLE request outcome is unknown")
	// ErrMessageTooLarge reports a request, response, or state message above the
	// configured memory limit.
	ErrMessageTooLarge = errors.New("BLE message exceeds the configured limit")
	// ErrProtocol reports malformed or unsupported BLE HTTP or FFE1 data.
	ErrProtocol = errors.New("invalid BUSY Bar BLE protocol message")

	errSensitiveHeader = errors.New("BLE requests must not contain authentication headers")
)

// Error reports a BLE transport, protocol, or lifecycle failure.
type Error struct {
	// Operation identifies the BLE or CoreBluetooth operation that failed.
	Operation string
	// NativeCode is the platform error code, or zero when none is available.
	NativeCode int
	// Err is the underlying cause exposed by Unwrap.
	Err error
}

// Error formats the BLE operation, native code when available, and cause.
func (e *Error) Error() string {
	if e.NativeCode != 0 && e.Err != nil {
		return fmt.Sprintf("BLE %s failed with native code %d: %v", e.Operation, e.NativeCode, e.Err)
	}
	if e.NativeCode != 0 {
		return fmt.Sprintf("BLE %s failed with native code %d", e.Operation, e.NativeCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("BLE %s failed: %v", e.Operation, e.Err)
	}
	return fmt.Sprintf("BLE %s failed", e.Operation)
}

// Unwrap returns the platform or protocol error that caused the BLE failure.
func (e *Error) Unwrap() error { return e.Err }
