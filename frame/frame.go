package frame

import (
	"fmt"
	"image"

	"github.com/lxdb/busylib-go/proto/framepb"
)

const (
	// FrontWidth is the front display width in pixels.
	FrontWidth = 72
	// FrontHeight is the front display height in pixels.
	FrontHeight = 16
	// BackWidth is the back display width in pixels.
	BackWidth = 160
	// BackHeight is the back display height in pixels.
	BackHeight = 80

	// MaxPayloadSize is the largest encoded or decoded frame in bytes.
	MaxPayloadSize = 16_384
)

// Frame preserves the status metadata and raw, possibly encoded, pixel data
// emitted by the BUSY Bar.
type Frame struct {
	Screen      framepb.Screen
	Width       uint32
	Height      uint32
	Encoding    framepb.Encoding
	PixelFormat framepb.PixelFormat
	Raw         []byte
}

// FromHTTP describes an uncompressed frame returned by GET /api/screen.
func FromHTTP(display int, raw []byte) (Frame, error) {
	var value Frame
	switch display {
	case int(framepb.Screen_FRONT):
		value = Frame{
			Screen:      framepb.Screen_FRONT,
			Width:       FrontWidth,
			Height:      FrontHeight,
			Encoding:    framepb.Encoding_PLAIN,
			PixelFormat: framepb.PixelFormat_RGB888,
		}
	case int(framepb.Screen_BACK):
		value = Frame{
			Screen:      framepb.Screen_BACK,
			Width:       BackWidth,
			Height:      BackHeight,
			Encoding:    framepb.Encoding_PLAIN,
			PixelFormat: framepb.PixelFormat_L4,
		}
	default:
		return Frame{}, frameError("from_http", Frame{Screen: framepb.Screen(display)}, ErrUnsupportedScreen)
	}

	want, err := value.pixelDataSize()
	if err != nil {
		return Frame{}, frameError("from_http", value, err)
	}
	if len(raw) != want {
		return Frame{}, frameError(
			"from_http",
			value,
			fmt.Errorf("%w: payload length is %d, want %d", ErrInvalidFrame, len(raw), want),
		)
	}
	value.Raw = append([]byte(nil), raw...)
	return value, nil
}

// FromProto preserves the metadata and encoded data from a status-stream frame.
// Unknown enum values remain available on the returned Frame and are rejected
// only when a conversion requires interpreting them.
func FromProto(value *framepb.Frame) (Frame, error) {
	if value == nil {
		return Frame{}, frameError("from_proto", Frame{}, ErrInvalidFrame)
	}
	result := Frame{
		Screen:      value.Screen,
		Width:       value.Width,
		Height:      value.Height,
		Encoding:    value.Encoding,
		PixelFormat: value.PixelFormat,
	}
	if len(value.Data) > MaxPayloadSize {
		return Frame{}, frameError("from_proto", result, ErrPayloadTooLarge)
	}
	result.Raw = append([]byte(nil), value.Data...)
	return result, nil
}

// Pixels returns a copy of the uncompressed bytes in the frame's pixel format.
// L4 output remains packed with the first pixel in the low nibble.
func (f Frame) Pixels() ([]byte, error) {
	return f.pixels("pixels")
}

// RGBA converts the frame into an opaque standard-library image.
func (f Frame) RGBA() (*image.RGBA, error) {
	pixels, err := f.pixels("rgba")
	if err != nil {
		return nil, err
	}

	width, height := int(f.Width), int(f.Height)
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	switch f.PixelFormat {
	case framepb.PixelFormat_RGB888:
		for pixel := 0; pixel < width*height; pixel++ {
			source := pixel * 3
			destination := pixel * 4
			result.Pix[destination] = pixels[source+2]
			result.Pix[destination+1] = pixels[source+1]
			result.Pix[destination+2] = pixels[source]
			result.Pix[destination+3] = 0xff
		}
	case framepb.PixelFormat_L8:
		for pixel, grayscale := range pixels {
			destination := pixel * 4
			result.Pix[destination] = grayscale
			result.Pix[destination+1] = grayscale
			result.Pix[destination+2] = grayscale
			result.Pix[destination+3] = 0xff
		}
	case framepb.PixelFormat_L4:
		for pixel := 0; pixel < width*height; pixel++ {
			packed := pixels[pixel/2]
			grayscale := packed & 0x0f
			if pixel%2 == 1 {
				grayscale = packed >> 4
			}
			grayscale *= 17
			destination := pixel * 4
			result.Pix[destination] = grayscale
			result.Pix[destination+1] = grayscale
			result.Pix[destination+2] = grayscale
			result.Pix[destination+3] = 0xff
		}
	default:
		return nil, frameError("rgba", f, ErrUnsupportedPixelFormat)
	}
	return result, nil
}

func (f Frame) pixels(operation string) ([]byte, error) {
	if err := f.validateMetadata(); err != nil {
		return nil, frameError(operation, f, err)
	}
	want, err := f.pixelDataSize()
	if err != nil {
		return nil, frameError(operation, f, err)
	}
	var pixels []byte
	switch f.Encoding {
	case framepb.Encoding_PLAIN:
		if len(f.Raw) != want {
			return nil, frameError(
				operation,
				f,
				fmt.Errorf("%w: decoded length is %d, want %d", ErrInvalidFrame, len(f.Raw), want),
			)
		}
		pixels = append([]byte(nil), f.Raw...)
	case framepb.Encoding_RUN_LENGTH:
		blockSize, err := f.rleBlockSize()
		if err != nil {
			return nil, frameError(operation, f, err)
		}
		pixels, err = decodeRLE(f.Raw, want, blockSize)
		if err != nil {
			return nil, frameError(operation, f, err)
		}
	default:
		return nil, frameError(operation, f, ErrUnsupportedEncoding)
	}
	return pixels, nil
}

func (f Frame) validateMetadata() error {
	if len(f.Raw) > MaxPayloadSize {
		return ErrPayloadTooLarge
	}
	switch f.Screen {
	case framepb.Screen_FRONT:
		if f.Width != FrontWidth || f.Height != FrontHeight {
			return fmt.Errorf("%w: front dimensions are %dx%d, want %dx%d", ErrInvalidFrame, f.Width, f.Height, FrontWidth, FrontHeight)
		}
	case framepb.Screen_BACK:
		if f.Width != BackWidth || f.Height != BackHeight {
			return fmt.Errorf("%w: back dimensions are %dx%d, want %dx%d", ErrInvalidFrame, f.Width, f.Height, BackWidth, BackHeight)
		}
	default:
		return ErrUnsupportedScreen
	}
	return nil
}

func (f Frame) pixelDataSize() (int, error) {
	pixelCount := uint64(f.Width) * uint64(f.Height)
	var size uint64
	switch f.PixelFormat {
	case framepb.PixelFormat_RGB888:
		size = pixelCount * 3
	case framepb.PixelFormat_L8:
		size = pixelCount
	case framepb.PixelFormat_L4:
		size = (pixelCount + 1) / 2
	default:
		return 0, ErrUnsupportedPixelFormat
	}
	if size > MaxPayloadSize {
		return 0, ErrPayloadTooLarge
	}
	return int(size), nil
}

func (f Frame) rleBlockSize() (int, error) {
	switch f.Screen {
	case framepb.Screen_FRONT:
		return 3, nil
	case framepb.Screen_BACK:
		return 2, nil
	default:
		return 0, ErrUnsupportedScreen
	}
}

func decodeRLE(source []byte, expectedSize, blockSize int) ([]byte, error) {
	result := make([]byte, 0, expectedSize)
	for sourceIndex := 0; sourceIndex < len(source); {
		opcode := source[sourceIndex]
		sourceIndex++
		blockCount := int(opcode & 0x7f)
		if blockCount == 0 {
			return nil, fmt.Errorf("%w: RLE opcode has a zero block count", ErrInvalidFrame)
		}

		byteCount := blockCount * blockSize
		if byteCount > expectedSize-len(result) {
			return nil, fmt.Errorf("%w: RLE output exceeds %d bytes", ErrInvalidFrame, expectedSize)
		}
		if opcode&0x80 != 0 {
			if byteCount > len(source)-sourceIndex {
				return nil, fmt.Errorf("%w: truncated RLE literal block", ErrInvalidFrame)
			}
			result = append(result, source[sourceIndex:sourceIndex+byteCount]...)
			sourceIndex += byteCount
			continue
		}

		if blockSize > len(source)-sourceIndex {
			return nil, fmt.Errorf("%w: truncated RLE repeat block", ErrInvalidFrame)
		}
		block := source[sourceIndex : sourceIndex+blockSize]
		for range blockCount {
			result = append(result, block...)
		}
		sourceIndex += blockSize
	}

	if len(result) != expectedSize {
		return nil, fmt.Errorf("%w: decoded length is %d, want %d", ErrInvalidFrame, len(result), expectedSize)
	}
	return result, nil
}

func frameError(operation string, f Frame, err error) *Error {
	return &Error{
		Operation:   operation,
		Screen:      f.Screen,
		Encoding:    f.Encoding,
		PixelFormat: f.PixelFormat,
		Err:         err,
	}
}
