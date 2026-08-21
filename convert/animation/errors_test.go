package animation

import (
	"bytes"
	"errors"
	"testing"
)

func TestEncodeRGB888RejectsInvalidInputsWithStableErrors(t *testing.T) {
	validFrame := RGB888Frame{PixelsBGR: []byte{1, 2, 3}}
	tests := []struct {
		name    string
		frames  []RGB888Frame
		config  RGB888Config
		options []Option
		want    error
	}{
		{name: "no frames", config: RGB888Config{Width: 1, Height: 1}, want: ErrNoFrames},
		{name: "zero width", frames: []RGB888Frame{validFrame}, config: RGB888Config{Height: 1}, want: ErrInvalidConfig},
		{name: "oversized height", frames: []RGB888Frame{validFrame}, config: RGB888Config{Width: 1, Height: 256}, want: ErrInvalidConfig},
		{name: "negative fps", frames: []RGB888Frame{validFrame}, config: RGB888Config{Width: 1, Height: 1, FPS: -1}, want: ErrInvalidConfig},
		{name: "oversized fps", frames: []RGB888Frame{validFrame}, config: RGB888Config{Width: 1, Height: 1, FPS: 256}, want: ErrInvalidConfig},
		{name: "frame exceeds uint16", frames: []RGB888Frame{{}}, config: RGB888Config{Width: 255, Height: 86}, want: ErrInvalidConfig},
		{name: "wrong frame length", frames: []RGB888Frame{{PixelsBGR: []byte{1, 2}}}, config: RGB888Config{Width: 1, Height: 1}, want: ErrInvalidFrame},
		{name: "output limit", frames: []RGB888Frame{validFrame}, config: RGB888Config{Width: 1, Height: 1}, options: []Option{WithMaxOutputBytes(63)}, want: ErrOutputTooLarge},
		{name: "invalid input limit", frames: []RGB888Frame{validFrame}, config: RGB888Config{Width: 1, Height: 1}, options: []Option{WithMaxInputBytes(0)}, want: ErrInvalidConfig},
		{name: "invalid output limit", frames: []RGB888Frame{validFrame}, config: RGB888Config{Width: 1, Height: 1}, options: []Option{WithMaxOutputBytes(0)}, want: ErrInvalidConfig},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := EncodeRGB888(test.frames, test.config, test.options...)
			if !errors.Is(err, test.want) {
				t.Fatalf("EncodeRGB888 error = %v, want errors.Is(_, %v)", err, test.want)
			}
			var conversionErr *ConversionError
			if !errors.As(err, &conversionErr) {
				t.Fatalf("EncodeRGB888 error type = %T, want *ConversionError", err)
			}
			if conversionErr.Operation == "" {
				t.Fatal("ConversionError.Operation is empty")
			}
			if test.name == "wrong frame length" && conversionErr.FrameIndex != 0 {
				t.Fatalf("ConversionError.FrameIndex = %d, want 0", conversionErr.FrameIndex)
			}
		})
	}
}

func TestEncodeRGB888IsDeterministicAndOwnsOutput(t *testing.T) {
	pixels := []byte{0x33, 0x22, 0x11}
	frames := []RGB888Frame{{PixelsBGR: pixels}}
	first, err := EncodeRGB888(frames, RGB888Config{Width: 1, Height: 1})
	if err != nil {
		t.Fatalf("first EncodeRGB888: %v", err)
	}
	second, err := EncodeRGB888(frames, RGB888Config{Width: 1, Height: 1})
	if err != nil {
		t.Fatalf("second EncodeRGB888: %v", err)
	}
	if !bytes.Equal(first.Data, second.Data) {
		t.Fatal("equivalent inputs produced different animation bytes")
	}

	pixels[0] = 0xff
	if !bytes.Equal(first.Data, second.Data) {
		t.Fatal("encoded output aliases caller pixel storage")
	}
	first.Data[len(first.Data)-1] = 0xee
	if bytes.Equal(first.Data, second.Data) {
		t.Fatal("separate results alias each other")
	}
}
