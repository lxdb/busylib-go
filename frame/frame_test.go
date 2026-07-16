package frame

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"

	"github.com/lxdb/busylib-go/proto/framepb"
)

func TestFromHTTPFrontPreservesRawAndConvertsBGRToRGBA(t *testing.T) {
	raw := make([]byte, FrontWidth*FrontHeight*3)
	raw[0], raw[1], raw[2] = 0x11, 0x22, 0x33

	got, err := FromHTTP(int(framepb.Screen_FRONT), raw)
	if err != nil {
		t.Fatalf("FromHTTP: %v", err)
	}
	if got.Screen != framepb.Screen_FRONT || got.Width != FrontWidth || got.Height != FrontHeight {
		t.Fatalf("front metadata = screen %v, %dx%d", got.Screen, got.Width, got.Height)
	}
	if got.Encoding != framepb.Encoding_PLAIN || got.PixelFormat != framepb.PixelFormat_RGB888 {
		t.Fatalf("front format = %v/%v", got.Encoding, got.PixelFormat)
	}

	raw[0] = 0xff
	if got.Raw[0] != 0x11 {
		t.Fatalf("frame aliases HTTP input: first byte = %#x", got.Raw[0])
	}

	pixels, err := got.Pixels()
	if err != nil {
		t.Fatalf("Pixels: %v", err)
	}
	pixels[0] = 0xee
	if got.Raw[0] != 0x11 {
		t.Fatalf("Pixels aliases raw frame: first byte = %#x", got.Raw[0])
	}

	rgba, err := got.RGBA()
	if err != nil {
		t.Fatalf("RGBA: %v", err)
	}
	if rgba.Bounds() != image.Rect(0, 0, FrontWidth, FrontHeight) {
		t.Fatalf("RGBA bounds = %v", rgba.Bounds())
	}
	if rgba.Stride != FrontWidth*4 {
		t.Fatalf("RGBA stride = %d", rgba.Stride)
	}
	if pixel := rgba.RGBAAt(0, 0); pixel != (color.RGBA{R: 0x33, G: 0x22, B: 0x11, A: 0xff}) {
		t.Fatalf("first pixel = %#v", pixel)
	}
}

func TestFromHTTPBackConvertsLowNibbleFirstL4ToRGBA(t *testing.T) {
	raw := make([]byte, BackWidth*BackHeight/2)
	raw[0] = 0xf1

	got, err := FromHTTP(int(framepb.Screen_BACK), raw)
	if err != nil {
		t.Fatalf("FromHTTP: %v", err)
	}
	if got.Screen != framepb.Screen_BACK || got.Width != BackWidth || got.Height != BackHeight {
		t.Fatalf("back metadata = screen %v, %dx%d", got.Screen, got.Width, got.Height)
	}
	if got.Encoding != framepb.Encoding_PLAIN || got.PixelFormat != framepb.PixelFormat_L4 {
		t.Fatalf("back format = %v/%v", got.Encoding, got.PixelFormat)
	}

	rgba, err := got.RGBA()
	if err != nil {
		t.Fatalf("RGBA: %v", err)
	}
	if first := rgba.RGBAAt(0, 0); first != (color.RGBA{R: 17, G: 17, B: 17, A: 0xff}) {
		t.Fatalf("first L4 pixel = %#v", first)
	}
	if second := rgba.RGBAAt(1, 0); second != (color.RGBA{R: 255, G: 255, B: 255, A: 0xff}) {
		t.Fatalf("second L4 pixel = %#v", second)
	}
}

func TestFromHTTPRejectsUnknownDisplayAndWrongPayloadLength(t *testing.T) {
	tests := []struct {
		name    string
		display int
		data    []byte
		want    error
	}{
		{name: "unknown display", display: 2, want: ErrUnsupportedScreen},
		{name: "short front", display: 0, data: make([]byte, FrontWidth*FrontHeight*3-1), want: ErrInvalidFrame},
		{name: "long back", display: 1, data: make([]byte, BackWidth*BackHeight/2+1), want: ErrInvalidFrame},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := FromHTTP(test.display, test.data)
			if !errors.Is(err, test.want) {
				t.Fatalf("FromHTTP error = %v, want %v", err, test.want)
			}
			var frameError *Error
			if !errors.As(err, &frameError) {
				t.Fatalf("FromHTTP error type = %T, want *frame.Error", err)
			}
			if frameError.Operation != "from_http" {
				t.Fatalf("error operation = %q", frameError.Operation)
			}
		})
	}
}

func TestFromProtoPreservesMetadataAndCopiesRawPayload(t *testing.T) {
	raw := []byte{1, 2, 3}
	value := &framepb.Frame{
		Screen:      framepb.Screen(9),
		Width:       7,
		Height:      5,
		Encoding:    framepb.Encoding(12),
		PixelFormat: framepb.PixelFormat(14),
		Data:        raw,
	}

	got, err := FromProto(value)
	if err != nil {
		t.Fatalf("FromProto: %v", err)
	}
	if got.Screen != value.Screen || got.Width != value.Width || got.Height != value.Height ||
		got.Encoding != value.Encoding || got.PixelFormat != value.PixelFormat {
		t.Fatalf("frame metadata = %#v, want %#v", got, value)
	}
	raw[0] = 0xff
	if got.Raw[0] != 1 {
		t.Fatalf("frame aliases protobuf input: first byte = %#x", got.Raw[0])
	}

	_, err = got.Pixels()
	if !errors.Is(err, ErrUnsupportedScreen) {
		t.Fatalf("Pixels error = %v, want unsupported screen", err)
	}
}

func TestFromProtoRejectsNilAndOversizedPayloads(t *testing.T) {
	tests := []struct {
		name  string
		value *framepb.Frame
		want  error
	}{
		{name: "nil", want: ErrInvalidFrame},
		{
			name: "oversized",
			value: &framepb.Frame{
				Screen:      framepb.Screen_FRONT,
				Width:       FrontWidth,
				Height:      FrontHeight,
				Encoding:    framepb.Encoding_RUN_LENGTH,
				PixelFormat: framepb.PixelFormat_RGB888,
				Data:        make([]byte, MaxPayloadSize+1),
			},
			want: ErrPayloadTooLarge,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := FromProto(test.value)
			if !errors.Is(err, test.want) {
				t.Fatalf("FromProto error = %v, want %v", err, test.want)
			}
			var frameError *Error
			if !errors.As(err, &frameError) || frameError.Operation != "from_proto" {
				t.Fatalf("FromProto error = %#v", err)
			}
		})
	}
}

func TestPixelsDecodesFrontFirmwareRLE(t *testing.T) {
	encoded := []byte{
		0x82,
		0x11, 0x22, 0x33,
		0x44, 0x55, 0x66,
	}
	encoded = appendRLERepeats(encoded, []byte{0xaa, 0xbb, 0xcc}, FrontWidth*FrontHeight-2)
	value, err := FromProto(&framepb.Frame{
		Screen:      framepb.Screen_FRONT,
		Width:       FrontWidth,
		Height:      FrontHeight,
		Encoding:    framepb.Encoding_RUN_LENGTH,
		PixelFormat: framepb.PixelFormat_RGB888,
		Data:        encoded,
	})
	if err != nil {
		t.Fatalf("FromProto: %v", err)
	}

	pixels, err := value.Pixels()
	if err != nil {
		t.Fatalf("Pixels: %v", err)
	}
	if len(pixels) != FrontWidth*FrontHeight*3 {
		t.Fatalf("decoded length = %d", len(pixels))
	}
	if !bytes.Equal(pixels[:9], []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0xaa, 0xbb, 0xcc}) {
		t.Fatalf("decoded prefix = %x", pixels[:9])
	}

	rgba, err := value.RGBA()
	if err != nil {
		t.Fatalf("RGBA: %v", err)
	}
	if first := rgba.RGBAAt(0, 0); first != (color.RGBA{R: 0x33, G: 0x22, B: 0x11, A: 0xff}) {
		t.Fatalf("first pixel = %#v", first)
	}
	if second := rgba.RGBAAt(1, 0); second != (color.RGBA{R: 0x66, G: 0x55, B: 0x44, A: 0xff}) {
		t.Fatalf("second pixel = %#v", second)
	}
}

func TestPixelsDecodesBackFirmwareRLEWithTwoByteBlocks(t *testing.T) {
	encoded := []byte{0x81, 0x21, 0x43}
	encoded = appendRLERepeats(encoded, []byte{0x65, 0x87}, BackWidth*BackHeight/4-1)
	value, err := FromProto(&framepb.Frame{
		Screen:      framepb.Screen_BACK,
		Width:       BackWidth,
		Height:      BackHeight,
		Encoding:    framepb.Encoding_RUN_LENGTH,
		PixelFormat: framepb.PixelFormat_L4,
		Data:        encoded,
	})
	if err != nil {
		t.Fatalf("FromProto: %v", err)
	}

	pixels, err := value.Pixels()
	if err != nil {
		t.Fatalf("Pixels: %v", err)
	}
	if len(pixels) != BackWidth*BackHeight/2 {
		t.Fatalf("decoded length = %d", len(pixels))
	}
	if !bytes.Equal(pixels[:4], []byte{0x21, 0x43, 0x65, 0x87}) {
		t.Fatalf("decoded prefix = %x", pixels[:4])
	}
}

func TestRGBAConvertsPlainL8(t *testing.T) {
	raw := make([]byte, FrontWidth*FrontHeight)
	raw[0] = 42
	value, err := FromProto(&framepb.Frame{
		Screen:      framepb.Screen_FRONT,
		Width:       FrontWidth,
		Height:      FrontHeight,
		Encoding:    framepb.Encoding_PLAIN,
		PixelFormat: framepb.PixelFormat_L8,
		Data:        raw,
	})
	if err != nil {
		t.Fatalf("FromProto: %v", err)
	}

	rgba, err := value.RGBA()
	if err != nil {
		t.Fatalf("RGBA: %v", err)
	}
	if first := rgba.RGBAAt(0, 0); first != (color.RGBA{R: 42, G: 42, B: 42, A: 0xff}) {
		t.Fatalf("first L8 pixel = %#v", first)
	}
}

func TestPixelsRejectsUnsupportedMetadataAndMalformedPayloads(t *testing.T) {
	valid := Frame{
		Screen:      framepb.Screen_FRONT,
		Width:       FrontWidth,
		Height:      FrontHeight,
		Encoding:    framepb.Encoding_RUN_LENGTH,
		PixelFormat: framepb.PixelFormat_RGB888,
	}
	overflow := make([]byte, 0, 40)
	for range 10 {
		overflow = append(overflow, 0x7f, 1, 2, 3)
	}

	tests := []struct {
		name  string
		value Frame
		want  error
	}{
		{name: "deflate", value: withEncoding(valid, framepb.Encoding_DEFLATE), want: ErrUnsupportedEncoding},
		{name: "deflate RLE", value: withEncoding(valid, framepb.Encoding_DEFLATE_RUN_LENGTH), want: ErrUnsupportedEncoding},
		{name: "unknown encoding", value: withEncoding(valid, framepb.Encoding(99)), want: ErrUnsupportedEncoding},
		{name: "unknown pixel format", value: withPixelFormat(valid, framepb.PixelFormat(99)), want: ErrUnsupportedPixelFormat},
		{name: "unknown screen", value: withScreen(valid, framepb.Screen(99)), want: ErrUnsupportedScreen},
		{name: "wrong dimensions", value: withWidth(valid, FrontWidth-1), want: ErrInvalidFrame},
		{name: "zero opcode", value: withRaw(valid, []byte{0}), want: ErrInvalidFrame},
		{name: "truncated literal", value: withRaw(valid, []byte{0x82, 1, 2, 3}), want: ErrInvalidFrame},
		{name: "truncated repeat", value: withRaw(valid, []byte{0x02, 1, 2}), want: ErrInvalidFrame},
		{name: "decoded overflow", value: withRaw(valid, overflow), want: ErrInvalidFrame},
		{name: "decoded underflow", value: withRaw(valid, []byte{0x01, 1, 2, 3}), want: ErrInvalidFrame},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.value.Pixels()
			if !errors.Is(err, test.want) {
				t.Fatalf("Pixels error = %v, want %v", err, test.want)
			}
			var frameError *Error
			if !errors.As(err, &frameError) || frameError.Operation != "pixels" {
				t.Fatalf("Pixels error = %#v", err)
			}
		})
	}
}

func FuzzPixelsRejectsMalformedFramesWithoutPanicking(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0})
	f.Add([]byte{0x82, 1, 2, 3})
	f.Add([]byte{0x7f, 1, 2, 3})
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > MaxPayloadSize {
			t.Skip()
		}
		value := Frame{
			Screen:      framepb.Screen_FRONT,
			Width:       FrontWidth,
			Height:      FrontHeight,
			Encoding:    framepb.Encoding_RUN_LENGTH,
			PixelFormat: framepb.PixelFormat_RGB888,
			Raw:         raw,
		}
		pixels, err := value.Pixels()
		if err == nil && len(pixels) != FrontWidth*FrontHeight*3 {
			t.Fatalf("decoded length = %d", len(pixels))
		}
	})
}

func appendRLERepeats(encoded, block []byte, blockCount int) []byte {
	for blockCount > 0 {
		count := min(blockCount, 127)
		encoded = append(encoded, byte(count))
		encoded = append(encoded, block...)
		blockCount -= count
	}
	return encoded
}

func withEncoding(value Frame, encoding framepb.Encoding) Frame {
	value.Encoding = encoding
	return value
}

func withPixelFormat(value Frame, pixelFormat framepb.PixelFormat) Frame {
	value.PixelFormat = pixelFormat
	return value
}

func withScreen(value Frame, screen framepb.Screen) Frame {
	value.Screen = screen
	return value
}

func withWidth(value Frame, width uint32) Frame {
	value.Width = width
	return value
}

func withRaw(value Frame, raw []byte) Frame {
	value.Raw = raw
	return value
}
