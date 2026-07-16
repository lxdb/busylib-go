package frame

import (
	"errors"
	"fmt"

	"github.com/lxdb/busylib-go/proto/framepb"
)

var (
	ErrInvalidFrame           = errors.New("invalid BUSY Bar frame")
	ErrPayloadTooLarge        = errors.New("BUSY Bar frame payload is too large")
	ErrUnsupportedScreen      = errors.New("unsupported BUSY Bar screen")
	ErrUnsupportedEncoding    = errors.New("unsupported BUSY Bar frame encoding")
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

func (e *Error) Unwrap() error {
	return e.Err
}
