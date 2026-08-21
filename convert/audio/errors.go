package audio

import (
	"errors"
	"fmt"
)

var (
	// ErrUnsupportedFormat reports a filename extension that Convert cannot process.
	ErrUnsupportedFormat = errors.New("unsupported audio conversion input")
	// ErrInvalidAudio reports empty or malformed device-ready PCM data.
	ErrInvalidAudio = errors.New("invalid BUSY Bar PCM audio")
	// ErrToolFailed reports that the external audio conversion process failed.
	ErrToolFailed = errors.New("audio conversion tool failed")
	// ErrOutputTooLarge reports PCM data that exceeds the configured byte limit.
	ErrOutputTooLarge = errors.New("audio output exceeds the configured limit")
)

// ConversionError describes audio validation or external-tool failure.
type ConversionError struct {
	Operation   string
	InputFormat string
	Tool        string
	Stderr      string
	Err         error
}

// Error describes the failed operation without including audio input data.
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

// Unwrap returns the underlying validation, tool, context, or I/O error.
func (e *ConversionError) Unwrap() error { return e.Err }
