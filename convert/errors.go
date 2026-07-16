package convert

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported image format")
	ErrAnimatedImage     = errors.New("animated GIF input is not supported")
	ErrInvalidTarget     = errors.New("invalid BUSY Bar display target")
	ErrInvalidImage      = errors.New("invalid image")
)

// ConversionError describes an image decode or preparation failure.
type ConversionError struct {
	Operation string
	Format    string
	Err       error
}

func (e *ConversionError) Error() string {
	if e.Format != "" {
		return fmt.Sprintf("image %s failed for %s input: %v", e.Operation, e.Format, e.Err)
	}
	return fmt.Sprintf("image %s failed: %v", e.Operation, e.Err)
}

func (e *ConversionError) Unwrap() error { return e.Err }

func conversionError(operation, format string, err error) error {
	return &ConversionError{Operation: operation, Format: format, Err: err}
}
