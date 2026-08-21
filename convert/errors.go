package convert

import (
	"errors"
	"fmt"
)

var (
	// ErrUnsupportedFormat reports an image format that Convert cannot decode.
	ErrUnsupportedFormat = errors.New("unsupported image format")
	// ErrAnimatedImage reports GIF input with more than one image frame.
	ErrAnimatedImage = errors.New("animated GIF input is not supported")
	// ErrInvalidTarget reports an unknown or inconsistent display target.
	ErrInvalidTarget = errors.New("invalid BUSY Bar display target")
	// ErrInvalidImage reports missing or malformed source image data.
	ErrInvalidImage = errors.New("invalid image")
	// ErrInputTooLarge reports encoded input that exceeds the configured byte limit.
	ErrInputTooLarge = errors.New("image input exceeds the configured limit")
	// ErrSourceImageTooLarge reports dimensions that exceed the configured pixel limit.
	ErrSourceImageTooLarge = errors.New("source image exceeds the configured pixel limit")
)

// ConversionError describes an image decode or preparation failure.
type ConversionError struct {
	Operation string
	Format    string
	Err       error
}

// Error describes the failed operation without including source image data.
func (e *ConversionError) Error() string {
	if e.Format != "" {
		return fmt.Sprintf("image %s failed for %s input: %v", e.Operation, e.Format, e.Err)
	}
	return fmt.Sprintf("image %s failed: %v", e.Operation, e.Err)
}

// Unwrap returns the underlying decode, validation, or I/O error.
func (e *ConversionError) Unwrap() error { return e.Err }

func conversionError(operation, format string, err error) error {
	return &ConversionError{Operation: operation, Format: format, Err: err}
}
