package animation

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidConfig reports dimensions, playback settings, or limits that
	// cannot be represented safely by the animation format.
	ErrInvalidConfig = errors.New("invalid animation configuration")
	// ErrNoFrames reports an empty animation sequence.
	ErrNoFrames = errors.New("animation contains no frames")
	// ErrInvalidFrame reports missing, malformed, or inconsistent frame data.
	ErrInvalidFrame = errors.New("invalid animation frame")
	// ErrInvalidMetadata reports malformed or incomplete ZIP metadata.
	ErrInvalidMetadata = errors.New("invalid animation metadata")
	// ErrUnsupportedColorMode reports a source color mode other than rgb888.
	ErrUnsupportedColorMode = errors.New("unsupported animation color mode")
	// ErrUnsupportedSections reports custom sections, which v1 does not encode.
	ErrUnsupportedSections = errors.New("custom animation sections are not supported")
	// ErrInputTooLarge reports ZIP input beyond the configured bound.
	ErrInputTooLarge = errors.New("animation input exceeds the configured limit")
	// ErrOutputTooLarge reports encoded output beyond the configured bound.
	ErrOutputTooLarge = errors.New("animation output exceeds the configured limit")
)

// ConversionError describes a safe animation conversion failure.
type ConversionError struct {
	Operation  string
	FrameIndex int
	Entry      string
	Err        error
}

// Error reports conversion context without including pixels or archive data.
func (e *ConversionError) Error() string {
	context := "animation " + e.Operation + " failed"
	if e.FrameIndex >= 0 {
		context += fmt.Sprintf(" at frame %d", e.FrameIndex)
	}
	if e.Entry != "" {
		context += fmt.Sprintf(" for archive entry %q", e.Entry)
	}
	return context + ": " + e.Err.Error()
}

// Unwrap returns the underlying validation, format, or I/O error.
func (e *ConversionError) Unwrap() error { return e.Err }

func conversionError(operation string, frameIndex int, entry string, err error) error {
	return &ConversionError{
		Operation:  operation,
		FrameIndex: frameIndex,
		Entry:      entry,
		Err:        err,
	}
}
