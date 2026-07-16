package convert

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	busylib "github.com/lxdb/busylib-go"
)

func TestImageDownscalesAndCenterCropsForDisplay(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 20, A: 255})
		}
	}
	var input bytes.Buffer
	if err := png.Encode(&input, source); err != nil {
		t.Fatalf("encode input: %v", err)
	}

	result, err := Image(bytes.NewReader(input.Bytes()), busylib.DisplayFront)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	if result.Width != 72 || result.Height != 16 || result.Format != "png" {
		t.Fatalf("result = %#v", result)
	}
	decoded, format, err := image.Decode(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if format != "png" || decoded.Bounds() != image.Rect(0, 0, 72, 16) {
		t.Fatalf("decoded format/bounds = %s/%v", format, decoded.Bounds())
	}
}

func TestImageDoesNotUpscaleSmallInputs(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 10, 5))
	var input bytes.Buffer
	if err := jpeg.Encode(&input, source, nil); err != nil {
		t.Fatalf("encode input: %v", err)
	}

	result, err := Image(bytes.NewReader(input.Bytes()), busylib.DisplayBack)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	if result.Width != 10 || result.Height != 5 || result.SourceFormat != "jpeg" {
		t.Fatalf("result = %#v", result)
	}
}

func TestImageAcceptsStaticGIFAndRejectsAnimatedGIF(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White})
	var static bytes.Buffer
	if err := gif.Encode(&static, frame, nil); err != nil {
		t.Fatalf("encode static GIF: %v", err)
	}
	if _, err := Image(bytes.NewReader(static.Bytes()), busylib.DisplayFront); err != nil {
		t.Fatalf("static GIF: %v", err)
	}

	var animated bytes.Buffer
	if err := gif.EncodeAll(&animated, &gif.GIF{Image: []*image.Paletted{frame, frame}, Delay: []int{1, 1}}); err != nil {
		t.Fatalf("encode animated GIF: %v", err)
	}
	if _, err := Image(bytes.NewReader(animated.Bytes()), busylib.DisplayFront); !errors.Is(err, ErrAnimatedImage) {
		t.Fatalf("animated GIF error = %v", err)
	}
}

func TestImageRejectsUnknownDataAndDisplay(t *testing.T) {
	if _, err := Image(nil, busylib.DisplayFront); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("nil source error = %v", err)
	}
	if _, err := Image(bytes.NewBufferString("not an image"), busylib.DisplayFront); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("unknown data error = %v", err)
	}
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatalf("encode input: %v", err)
	}
	if _, err := Image(bytes.NewReader(input.Bytes()), busylib.DisplayTarget("side")); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("invalid target error = %v", err)
	}
}
