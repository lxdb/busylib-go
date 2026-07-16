package audio

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported audio conversion input")
	ErrInvalidAudio      = errors.New("invalid BUSY Bar PCM audio")
	ErrToolFailed        = errors.New("audio conversion tool failed")
)

// ConversionError describes audio validation or external-tool failure.
type ConversionError struct {
	Operation   string
	InputFormat string
	Tool        string
	Stderr      string
	Err         error
}

func (e *ConversionError) Error() string {
	message := fmt.Sprintf("audio %s failed", e.Operation)
	if e.InputFormat != "" {
		message += " for " + e.InputFormat + " input"
	}
	if e.Tool != "" {
		message += " using " + e.Tool
	}
	if e.Stderr != "" {
		return fmt.Sprintf("%s: %v: %s", message, e.Err, e.Stderr)
	}
	return fmt.Sprintf("%s: %v", message, e.Err)
}

func (e *ConversionError) Unwrap() error { return e.Err }
