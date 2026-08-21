package animation

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

const (
	// Extension is the filename extension for device-native animations.
	Extension = ".anim"
	// DefaultFPS is used when an encoder receives an FPS value of zero.
	DefaultFPS = 30
	// DefaultMaxInputBytes bounds ZIP input buffered in memory.
	DefaultMaxInputBytes int64 = 32 << 20
	// DefaultMaxOutputBytes bounds encoded animation data buffered in memory.
	DefaultMaxOutputBytes int64 = 64 << 20

	signature     = "bicycle0"
	headerLength  = 36
	sectionName   = "default"
	colorRGB888   = 0
	encodingRaw   = 0
	maxByteValue  = 255
	maxFrameBytes = 65535
)

// RGB888Frame contains one firmware RGB888 frame. Despite the format name,
// BUSY Bar stores each pixel in B, G, R byte order.
type RGB888Frame struct {
	PixelsBGR []byte
	Duration  uint8
}

// RGB888Config describes the dimensions and playback rate of raw frames.
type RGB888Config struct {
	Width  int
	Height int
	FPS    int
}

// Result owns a device-native animation and its normalized metadata.
type Result struct {
	Data              []byte
	Width             int
	Height            int
	FPS               int
	EncodedFrameCount int
	DisplayFrameCount int
}

type config struct {
	maxInputBytes  int64
	maxOutputBytes int64
}

type animationLayout struct {
	sectionsLength int
	framesLength   int
	outputLength   int
}

// Option configures animation conversion limits.
type Option func(*config) error

// WithMaxInputBytes changes the ZIP input and relevant expanded-entry limit.
func WithMaxInputBytes(maximum int64) Option {
	return func(config *config) error {
		if maximum <= 0 {
			return fmt.Errorf("%w: maximum input size must be greater than zero", ErrInvalidConfig)
		}
		config.maxInputBytes = maximum
		return nil
	}
}

// WithMaxOutputBytes changes the encoded animation output limit.
func WithMaxOutputBytes(maximum int64) Option {
	return func(config *config) error {
		if maximum <= 0 {
			return fmt.Errorf("%w: maximum output size must be greater than zero", ErrInvalidConfig)
		}
		config.maxOutputBytes = maximum
		return nil
	}
}

// EncodeRGB888 packages BGR-ordered RGB888 frames as a firmware-native
// bicycle0 animation.
func EncodeRGB888(frames []RGB888Frame, format RGB888Config, options ...Option) (Result, error) {
	conversionConfig, err := newConfig(options)
	if err != nil {
		return Result{}, err
	}
	if format.FPS == 0 {
		format.FPS = DefaultFPS
	}
	if format.Width <= 0 || format.Width > maxByteValue || format.Height <= 0 || format.Height > maxByteValue {
		return Result{}, conversionError("configure", -1, "", fmt.Errorf(
			"%w: dimensions must be between 1 and %d", ErrInvalidConfig, maxByteValue,
		))
	}
	if format.FPS <= 0 || format.FPS > maxByteValue {
		return Result{}, conversionError("configure", -1, "", fmt.Errorf(
			"%w: fps must be between 1 and %d", ErrInvalidConfig, maxByteValue,
		))
	}
	if len(frames) == 0 {
		return Result{}, conversionError("validate", -1, "", ErrNoFrames)
	}
	frameLength := format.Width * format.Height * 3
	if frameLength > maxFrameBytes {
		return Result{}, conversionError("configure", -1, "", fmt.Errorf(
			"%w: frame length %d exceeds %d", ErrInvalidConfig, frameLength, maxFrameBytes,
		))
	}
	displayFrameCount := uint64(0)
	for index, frame := range frames {
		if len(frame.PixelsBGR) != frameLength {
			return Result{}, conversionError("validate", index, "", fmt.Errorf(
				"%w: has %d bytes, want %d", ErrInvalidFrame, len(frame.PixelsBGR), frameLength,
			))
		}
		displayFrameCount += uint64(effectiveDuration(frame.Duration))
	}
	normalizedFrames := coalesceFrames(frames)
	layout, err := calculateLayout(
		uint64(frameLength),
		uint64(len(normalizedFrames)),
		displayFrameCount,
		conversionConfig.maxOutputBytes,
	)
	if err != nil {
		return Result{}, conversionError("encode", -1, "", err)
	}
	output := make([]byte, layout.outputLength)
	copy(output[0:8], signature)
	output[9] = byte(format.Width)
	output[10] = byte(format.Height)
	output[11] = colorRGB888
	output[12] = byte(format.FPS)
	binary.LittleEndian.PutUint16(output[13:15], uint16(frameLength))
	binary.LittleEndian.PutUint32(output[16:20], uint32(layout.sectionsLength))
	binary.LittleEndian.PutUint32(output[20:24], uint32(layout.framesLength))
	binary.LittleEndian.PutUint32(output[24:28], 1)
	binary.LittleEndian.PutUint32(output[28:32], uint32(len(normalizedFrames)))
	binary.LittleEndian.PutUint32(output[32:36], uint32(displayFrameCount))

	cursor := headerLength
	firstFrameOffset := headerLength + layout.sectionsLength
	binary.LittleEndian.PutUint32(output[cursor:cursor+4], 0)
	binary.LittleEndian.PutUint32(output[cursor+4:cursor+8], uint32(displayFrameCount-1))
	binary.LittleEndian.PutUint32(output[cursor+8:cursor+12], uint32(firstFrameOffset))
	output[cursor+12] = normalizedFrames[0].Duration
	cursor += 13
	copy(output[cursor:], sectionName)
	cursor += len(sectionName) + 1

	for _, frame := range normalizedFrames {
		output[cursor] = encodingRaw
		output[cursor+1] = frame.Duration
		binary.LittleEndian.PutUint16(output[cursor+2:cursor+4], uint16(frameLength))
		cursor += 4
		copy(output[cursor:cursor+frameLength], frame.PixelsBGR)
		cursor += frameLength
	}

	return Result{
		Data:              output,
		Width:             format.Width,
		Height:            format.Height,
		FPS:               format.FPS,
		EncodedFrameCount: len(normalizedFrames),
		DisplayFrameCount: int(displayFrameCount),
	}, nil
}

func calculateLayout(frameLength, frameCount, displayFrameCount uint64, maximum int64) (animationLayout, error) {
	sectionsLength := uint64(13 + len(sectionName) + 1)
	if frameCount > math.MaxUint32 {
		return animationLayout{}, fmt.Errorf("%w: encoded frame count exceeds uint32", ErrOutputTooLarge)
	}
	maxInt := uint64(^uint(0) >> 1)
	if displayFrameCount > math.MaxUint32 || displayFrameCount > maxInt {
		return animationLayout{}, fmt.Errorf("%w: display frame count cannot be represented", ErrOutputTooLarge)
	}
	recordLength := uint64(4) + frameLength
	if frameCount != 0 && recordLength > math.MaxUint64/frameCount {
		return animationLayout{}, fmt.Errorf("%w: frame chunk length overflow", ErrOutputTooLarge)
	}
	framesLength := recordLength * frameCount
	if framesLength > math.MaxUint32 {
		return animationLayout{}, fmt.Errorf("%w: frame chunk length exceeds uint32", ErrOutputTooLarge)
	}
	outputLength := uint64(headerLength) + sectionsLength + framesLength
	if outputLength > maxInt || outputLength > uint64(maximum) {
		return animationLayout{}, fmt.Errorf(
			"%w: output has %d bytes, limit is %d", ErrOutputTooLarge, outputLength, maximum,
		)
	}
	return animationLayout{
		sectionsLength: int(sectionsLength),
		framesLength:   int(framesLength),
		outputLength:   int(outputLength),
	}, nil
}

func newConfig(options []Option) (config, error) {
	conversionConfig := config{
		maxInputBytes:  DefaultMaxInputBytes,
		maxOutputBytes: DefaultMaxOutputBytes,
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&conversionConfig); err != nil {
			return config{}, conversionError("configure", -1, "", err)
		}
	}
	return conversionConfig, nil
}

func coalesceFrames(frames []RGB888Frame) []RGB888Frame {
	normalized := make([]RGB888Frame, 0, len(frames))
	for _, frame := range frames {
		remaining := int(effectiveDuration(frame.Duration))
		if len(normalized) > 0 && bytes.Equal(normalized[len(normalized)-1].PixelsBGR, frame.PixelsBGR) {
			last := &normalized[len(normalized)-1]
			available := maxByteValue - int(last.Duration)
			added := min(remaining, available)
			last.Duration += uint8(added)
			remaining -= added
		}
		for remaining > 0 {
			duration := min(remaining, maxByteValue)
			normalized = append(normalized, RGB888Frame{
				PixelsBGR: frame.PixelsBGR,
				Duration:  uint8(duration),
			})
			remaining -= duration
		}
	}
	return normalized
}

func effectiveDuration(duration uint8) uint8 {
	if duration == 0 {
		return 1
	}
	return duration
}
