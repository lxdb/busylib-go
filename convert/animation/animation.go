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
	// Duration is the number of display frames to retain this image. Zero means
	// one display frame.
	Duration uint8
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

type frameLoader func(index int, pixels []byte) (duration uint8, entry string, err error)

// Option configures animation conversion limits. Encoders ignore nil options.
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
// bicycle0 animation. A zero FPS uses DefaultFPS. The result does not retain
// the input slices.
func EncodeRGB888(frames []RGB888Frame, format RGB888Config, options ...Option) (Result, error) {
	conversionConfig, err := newConfig(options)
	if err != nil {
		return Result{}, err
	}
	return encodeFrames(len(frames), format, conversionConfig, func(index int, pixels []byte) (uint8, string, error) {
		frame := frames[index]
		if len(frame.PixelsBGR) != len(pixels) {
			return 0, "", conversionError("validate", index, "", fmt.Errorf(
				"%w: has %d bytes, want %d", ErrInvalidFrame, len(frame.PixelsBGR), len(pixels),
			))
		}
		copy(pixels, frame.PixelsBGR)
		return frame.Duration, "", nil
	})
}

func encodeFrames(frameCount int, format RGB888Config, conversionConfig config, load frameLoader) (Result, error) {
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
	if frameCount == 0 {
		return Result{}, conversionError("validate", -1, "", ErrNoFrames)
	}
	frameLength := format.Width * format.Height * 3
	if frameLength > maxFrameBytes {
		return Result{}, conversionError("configure", -1, "", fmt.Errorf(
			"%w: frame length %d exceeds %d", ErrInvalidConfig, frameLength, maxFrameBytes,
		))
	}
	layout, encodedFrameCount, displayFrameCount, err := analyzeFrames(
		frameCount,
		frameLength,
		conversionConfig.maxOutputBytes,
		load,
	)
	if err != nil {
		return Result{}, err
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
	binary.LittleEndian.PutUint32(output[28:32], uint32(encodedFrameCount))
	binary.LittleEndian.PutUint32(output[32:36], uint32(displayFrameCount))

	cursor := headerLength
	firstFrameOffset := headerLength + layout.sectionsLength
	binary.LittleEndian.PutUint32(output[cursor:cursor+4], 0)
	binary.LittleEndian.PutUint32(output[cursor+4:cursor+8], uint32(displayFrameCount-1))
	binary.LittleEndian.PutUint32(output[cursor+8:cursor+12], uint32(firstFrameOffset))
	cursor += 13
	copy(output[cursor:], sectionName)
	cursor += len(sectionName) + 1

	if err := writeFrames(output, cursor, frameCount, frameLength, load); err != nil {
		return Result{}, err
	}
	output[headerLength+12] = output[firstFrameOffset+1]

	return Result{
		Data:              output,
		Width:             format.Width,
		Height:            format.Height,
		FPS:               format.FPS,
		EncodedFrameCount: int(encodedFrameCount),
		DisplayFrameCount: int(displayFrameCount),
	}, nil
}

func analyzeFrames(frameCount, frameLength int, maximum int64, load frameLoader) (animationLayout, uint64, uint64, error) {
	pixels := make([]byte, frameLength)
	previous := make([]byte, frameLength)
	havePrevious := false
	lastDuration := 0
	encodedFrameCount := uint64(0)
	displayFrameCount := uint64(0)
	var layout animationLayout
	for index := range frameCount {
		duration, entry, err := load(index, pixels)
		if err != nil {
			return animationLayout{}, 0, 0, err
		}
		remaining := int(effectiveDuration(duration))
		displayFrameCount += uint64(remaining)
		same := havePrevious && bytes.Equal(previous, pixels)
		if same {
			added := min(remaining, maxByteValue-lastDuration)
			lastDuration += added
			remaining -= added
		}
		for remaining > 0 {
			lastDuration = min(remaining, maxByteValue)
			remaining -= lastDuration
			encodedFrameCount++
		}
		if !same {
			copy(previous, pixels)
			havePrevious = true
		}
		layout, err = calculateLayout(uint64(frameLength), encodedFrameCount, displayFrameCount, maximum)
		if err != nil {
			return animationLayout{}, 0, 0, conversionError("encode", index, entry, err)
		}
	}
	return layout, encodedFrameCount, displayFrameCount, nil
}

func writeFrames(output []byte, cursor, frameCount, frameLength int, load frameLoader) error {
	pixels := make([]byte, frameLength)
	previous := make([]byte, frameLength)
	havePrevious := false
	lastFrameOffset := -1
	for index := range frameCount {
		duration, entry, err := load(index, pixels)
		if err != nil {
			return err
		}
		remaining := int(effectiveDuration(duration))
		same := havePrevious && bytes.Equal(previous, pixels)
		if same {
			available := maxByteValue - int(output[lastFrameOffset+1])
			added := min(remaining, available)
			output[lastFrameOffset+1] += byte(added)
			remaining -= added
		}
		for remaining > 0 {
			if cursor+4+frameLength > len(output) {
				return conversionError("encode", index, entry, fmt.Errorf("%w: frames changed during encoding", ErrInvalidFrame))
			}
			duration := min(remaining, maxByteValue)
			lastFrameOffset = cursor
			output[cursor] = encodingRaw
			output[cursor+1] = byte(duration)
			binary.LittleEndian.PutUint16(output[cursor+2:cursor+4], uint16(frameLength))
			cursor += 4
			copy(output[cursor:cursor+frameLength], pixels)
			cursor += frameLength
			remaining -= duration
		}
		if !same {
			copy(previous, pixels)
			havePrevious = true
		}
	}
	if cursor != len(output) {
		return conversionError("encode", -1, "", fmt.Errorf("%w: frames changed during encoding", ErrInvalidFrame))
	}
	return nil
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

func effectiveDuration(duration uint8) uint8 {
	if duration == 0 {
		return 1
	}
	return duration
}
