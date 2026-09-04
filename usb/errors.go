package usb

import (
	"errors"
	"fmt"
)

var (
	// ErrClosed reports an operation on a closed or failed persistent session.
	ErrClosed = errors.New("USB CLI session is closed")
	// ErrInvalidCommand reports an empty command or disallowed control character.
	ErrInvalidCommand = errors.New("invalid USB CLI command")
	// ErrResponseTooLarge reports a response that exceeds the configured byte limit.
	ErrResponseTooLarge = errors.New("USB CLI response exceeds the configured limit")
	// ErrPromptNotFound reports that input ended before the firmware prompt arrived.
	ErrPromptNotFound = errors.New("USB CLI prompt not found")
)

// Error describes a USB CLI connection, command, or lifecycle failure. Use
// errors.As to inspect it and errors.Is to inspect the wrapped cause. Command
// can contain user-supplied arguments and should not be logged blindly.
type Error struct {
	Operation string
	Address   string
	Command   string
	Err       error
}

// Error describes the failed operation and omits response data.
func (e *Error) Error() string {
	if e.Command != "" {
		return fmt.Sprintf("USB CLI %s %s command %q failed: %v", e.Operation, e.Address, e.Command, e.Err)
	}
	return fmt.Sprintf("USB CLI %s %s failed: %v", e.Operation, e.Address, e.Err)
}

// Unwrap returns the underlying context, network, protocol, or validation error.
func (e *Error) Unwrap() error { return e.Err }

func wrapError(operation, address, command string, err error) error {
	if err == nil {
		return nil
	}
	var cliError *Error
	if errors.As(err, &cliError) {
		return err
	}
	return &Error{Operation: operation, Address: address, Command: command, Err: err}
}
