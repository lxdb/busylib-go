package frame

import (
	"errors"
	"fmt"

	"github.com/lxdb/busylib-go/proto/framepb"
)

var (
	// ErrInvalidFrame reports inconsistent metadata or pixel data.
	ErrInvalidFrame = errors.New("invalid BUSY Bar frame")
	// ErrPayloadTooLarge reports frame data that exceeds MaxPayloadSize.
	ErrPayloadTooLarge = errors.New("BUSY Bar frame payload is too large")
	// ErrUnsupportedScreen reports an unknown firmware screen value.
	ErrUnsupportedScreen = errors.New("unsupported BUSY Bar screen")
	// ErrUnsupportedEncoding reports an encoding that Pixels cannot decode.
	ErrUnsupportedEncoding = errors.New("unsupported BUSY Bar frame encoding")
	// ErrUnsupportedPixelFormat reports a pixel format that cannot be sized or rendered.
	ErrUnsupportedPixelFormat = errors.New("unsupported BUSY Bar pixel format")
)

// Error describes a frame construction or conversion failure while preserving
// the raw protocol values that caused it.
type Error struct {
	Operation   string
	Screen      framepb.Screen
	Encoding    framepb.Encoding
	PixelFormat framepb.PixelFormat
	Err         error
}

// Error describes the operation and preserves unknown protocol enum values.
func (e *Error) Error() string {
	return fmt.Sprintf(
		"BUSY Bar frame %s failed (screen=%d encoding=%d pixel_format=%d): %v",
		e.Operation,
		e.Screen,
		e.Encoding,
		e.PixelFormat,
		e.Err,
	)
}

// Unwrap returns the underlying validation or conversion error.
func (e *Error) Unwrap() error {
	return e.Err
}
