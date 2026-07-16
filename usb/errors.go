package usb

import (
	"errors"
	"fmt"
)

var (
	ErrClosed           = errors.New("USB CLI session is closed")
	ErrInvalidCommand   = errors.New("invalid USB CLI command")
	ErrResponseTooLarge = errors.New("USB CLI response exceeds the configured limit")
	ErrPromptNotFound   = errors.New("USB CLI prompt not found")
)

// Error describes a USB CLI connection, command, or lifecycle failure.
type Error struct {
	Operation string
	Address   string
	Command   string
	Err       error
}

func (e *Error) Error() string {
	if e.Command != "" {
		return fmt.Sprintf("USB CLI %s %s command %q failed: %v", e.Operation, e.Address, e.Command, e.Err)
	}
	return fmt.Sprintf("USB CLI %s %s failed: %v", e.Operation, e.Address, e.Err)
}

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
