package animation

import (
	"fmt"
	"image"
	"image/color"
	"reflect"
)

// ImageFrame contains one standard-library image and its display duration.
// Alpha is ignored because firmware RGB888 animations do not store it.
type ImageFrame struct {
	Image    image.Image
	Duration uint8
}

// EncodeImages converts equal-sized images to firmware BGR byte order and
// packages them as a device-native animation. Images are never resized.
func EncodeImages(frames []ImageFrame, fps int, options ...Option) (Result, error) {
	if len(frames) == 0 {
		return Result{}, conversionError("validate", -1, "", ErrNoFrames)
	}
	if imageIsNil(frames[0].Image) {
		return Result{}, conversionError("validate", 0, "", ErrInvalidFrame)
	}
	firstBounds := frames[0].Image.Bounds()
	width, height := firstBounds.Dx(), firstBounds.Dy()
	if width <= 0 || height <= 0 {
		return Result{}, conversionError("validate", 0, "", fmt.Errorf(
			"%w: dimensions are %dx%d", ErrInvalidFrame, width, height,
		))
	}
	if width > maxByteValue || height > maxByteValue || width*height*3 > maxFrameBytes {
		return Result{}, conversionError("validate", 0, "", fmt.Errorf(
			"%w: dimensions %dx%d cannot be represented as RGB888", ErrInvalidFrame, width, height,
		))
	}

	for index, frame := range frames {
		if imageIsNil(frame.Image) {
			return Result{}, conversionError("validate", index, "", ErrInvalidFrame)
		}
		bounds := frame.Image.Bounds()
		if bounds.Dx() != width || bounds.Dy() != height {
			return Result{}, conversionError("validate", index, "", fmt.Errorf(
				"%w: dimensions are %dx%d, want %dx%d", ErrInvalidFrame, bounds.Dx(), bounds.Dy(), width, height,
			))
		}
	}

	conversionConfig, err := newConfig(options)
	if err != nil {
		return Result{}, err
	}
	return encodeFrames(
		len(frames),
		RGB888Config{Width: width, Height: height, FPS: fps},
		conversionConfig,
		func(index int, pixels []byte) (uint8, string, error) {
			fillImagePixels(pixels, frames[index].Image)
			return frames[index].Duration, "", nil
		},
	)
}

func fillImagePixels(pixels []byte, source image.Image) {
	bounds := source.Bounds()
	index := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := color.NRGBAModel.Convert(source.At(x, y)).(color.NRGBA)
			pixels[index] = pixel.B
			pixels[index+1] = pixel.G
			pixels[index+2] = pixel.R
			index += 3
		}
	}
}

func imageIsNil(value image.Image) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	return reflected.Kind() == reflect.Pointer && reflected.IsNil()
}
