package animation

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"
)

func TestEncodeImagesMatchesRawBGRAndHandlesNonZeroBounds(t *testing.T) {
	source := image.NewNRGBA(image.Rect(3, 4, 4, 5))
	source.SetNRGBA(3, 4, color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0})

	fromImage, err := EncodeImages([]ImageFrame{{Image: source}}, 30)
	if err != nil {
		t.Fatalf("EncodeImages: %v", err)
	}
	fromRaw, err := EncodeRGB888(
		[]RGB888Frame{{PixelsBGR: []byte{0x33, 0x22, 0x11}}},
		RGB888Config{Width: 1, Height: 1, FPS: 30},
	)
	if err != nil {
		t.Fatalf("EncodeRGB888: %v", err)
	}
	if !bytes.Equal(fromImage.Data, fromRaw.Data) {
		t.Fatalf("image and raw encoders differ:\nimage %x\nraw   %x", fromImage.Data, fromRaw.Data)
	}
}

func TestEncodeImagesRejectsNilAndMismatchedFrames(t *testing.T) {
	var typedNil *image.NRGBA
	tests := []struct {
		name   string
		frames []ImageFrame
		index  int
	}{
		{name: "nil image", frames: []ImageFrame{{}}, index: 0},
		{name: "typed nil image", frames: []ImageFrame{{Image: typedNil}}, index: 0},
		{
			name: "mismatched dimensions",
			frames: []ImageFrame{
				{Image: image.NewNRGBA(image.Rect(0, 0, 1, 1))},
				{Image: image.NewNRGBA(image.Rect(0, 0, 2, 1))},
			},
			index: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := EncodeImages(test.frames, 30)
			if !errors.Is(err, ErrInvalidFrame) {
				t.Fatalf("EncodeImages error = %v, want ErrInvalidFrame", err)
			}
			var conversionErr *ConversionError
			if !errors.As(err, &conversionErr) || conversionErr.FrameIndex != test.index {
				t.Fatalf("EncodeImages error = %#v, want frame index %d", conversionErr, test.index)
			}
		})
	}
}
