package convert

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"math"
	"os"

	busylib "github.com/lxdb/busylib-go"
	framepkg "github.com/lxdb/busylib-go/frame"
)

const (
	// DefaultMaxInputBytes bounds encoded image data buffered in memory.
	DefaultMaxInputBytes int64 = 32 << 20
	// DefaultMaxSourcePixels bounds decoded source image dimensions.
	DefaultMaxSourcePixels int64 = 4096 * 4096
)

type imageConfig struct {
	maxInputBytes   int64
	maxSourcePixels int64
}

// Option configures image conversion.
type Option func(*imageConfig) error

// WithMaxInputBytes changes the encoded image input limit.
func WithMaxInputBytes(maximum int64) Option {
	return func(config *imageConfig) error {
		if maximum <= 0 {
			return errors.New("maximum image input size must be greater than zero")
		}
		config.maxInputBytes = maximum
		return nil
	}
}

// WithMaxSourcePixels changes the decoded source pixel limit.
func WithMaxSourcePixels(maximum int64) Option {
	return func(config *imageConfig) error {
		if maximum <= 0 {
			return errors.New("maximum source pixel count must be greater than zero")
		}
		config.maxSourcePixels = maximum
		return nil
	}
}

// ImageResult is an owned PNG payload and its prepared dimensions.
type ImageResult struct {
	Data         []byte
	Width        int
	Height       int
	Format       string
	SourceFormat string
}

// Image decodes PNG, JPEG, or static GIF input, bilinearly downsizes it when
// necessary, center-crops it to the selected display's maximum dimensions,
// and returns a PNG payload. Images are never upscaled.
func Image(source io.Reader, target busylib.DisplayTarget, options ...Option) (ImageResult, error) {
	config := imageConfig{
		maxInputBytes:   DefaultMaxInputBytes,
		maxSourcePixels: DefaultMaxSourcePixels,
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&config); err != nil {
			return ImageResult{}, conversionError("configure", "", err)
		}
	}
	maximumWidth, maximumHeight, err := targetDimensions(target)
	if err != nil {
		return ImageResult{}, conversionError("prepare", "", err)
	}
	if source == nil {
		return ImageResult{}, conversionError("read", "", ErrInvalidImage)
	}
	reader := io.Reader(source)
	if config.maxInputBytes < math.MaxInt64 {
		reader = io.LimitReader(source, config.maxInputBytes+1)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return ImageResult{}, conversionError("read", "", fmt.Errorf("%w: %w", ErrInvalidImage, err))
	}
	if int64(len(data)) > config.maxInputBytes {
		return ImageResult{}, conversionError("read", "", ErrInputTooLarge)
	}
	decoded, sourceFormat, err := decodeImage(data, config.maxSourcePixels)
	if err != nil {
		return ImageResult{}, err
	}
	prepared, err := resizeAndCrop(decoded, maximumWidth, maximumHeight)
	if err != nil {
		return ImageResult{}, conversionError("prepare", sourceFormat, err)
	}
	var output bytes.Buffer
	if err := png.Encode(&output, prepared); err != nil {
		return ImageResult{}, conversionError("encode", sourceFormat, err)
	}
	resultData := append([]byte(nil), output.Bytes()...)
	return ImageResult{
		Data:         resultData,
		Width:        prepared.Bounds().Dx(),
		Height:       prepared.Bounds().Dy(),
		Format:       "png",
		SourceFormat: sourceFormat,
	}, nil
}

// ImageFile prepares an image read from path.
func ImageFile(path string, target busylib.DisplayTarget, options ...Option) (ImageResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return ImageResult{}, conversionError("open", "", err)
	}
	defer func() { _ = file.Close() }()
	return Image(file, target, options...)
}

func decodeImage(data []byte, maxSourcePixels int64) (image.Image, string, error) {
	configuration, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", conversionError("decode", "", fmt.Errorf("%w: %w", ErrUnsupportedFormat, err))
	}
	if configuration.Width <= 0 || configuration.Height <= 0 {
		return nil, format, conversionError("decode", format, ErrInvalidImage)
	}
	if int64(configuration.Width) > maxSourcePixels/int64(configuration.Height) {
		return nil, format, conversionError("decode", format, ErrSourceImageTooLarge)
	}
	switch format {
	case "png", "jpeg":
		decoded, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, format, conversionError("decode", format, fmt.Errorf("%w: %w", ErrInvalidImage, err))
		}
		return decoded, format, nil
	case "gif":
		frameCount, err := gifFrameCount(data)
		if err != nil {
			return nil, format, conversionError("decode", format, fmt.Errorf("%w: %w", ErrInvalidImage, err))
		}
		if frameCount != 1 {
			return nil, format, conversionError("decode", format, ErrAnimatedImage)
		}
		decoded, err := gif.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, format, conversionError("decode", format, fmt.Errorf("%w: %w", ErrInvalidImage, err))
		}
		return decoded, format, nil
	default:
		return nil, format, conversionError("decode", format, ErrUnsupportedFormat)
	}
}

func gifFrameCount(data []byte) (int, error) {
	if len(data) < 13 || (string(data[:6]) != "GIF87a" && string(data[:6]) != "GIF89a") {
		return 0, ErrInvalidImage
	}
	index := 13
	if data[10]&0x80 != 0 {
		index += 3 * (1 << ((data[10] & 0x07) + 1))
	}
	frames := 0
	for index < len(data) {
		switch data[index] {
		case 0x2c:
			frames++
			if frames > 1 {
				return frames, nil
			}
			if index+10 > len(data) {
				return 0, io.ErrUnexpectedEOF
			}
			packed := data[index+9]
			index += 10
			if packed&0x80 != 0 {
				index += 3 * (1 << ((packed & 0x07) + 1))
			}
			if index >= len(data) {
				return 0, io.ErrUnexpectedEOF
			}
			index++
			next, err := skipGIFSubBlocks(data, index)
			if err != nil {
				return 0, err
			}
			index = next
		case 0x21:
			if index+2 > len(data) {
				return 0, io.ErrUnexpectedEOF
			}
			next, err := skipGIFSubBlocks(data, index+2)
			if err != nil {
				return 0, err
			}
			index = next
		case 0x3b:
			return frames, nil
		default:
			return 0, ErrInvalidImage
		}
	}
	return 0, io.ErrUnexpectedEOF
}

func skipGIFSubBlocks(data []byte, index int) (int, error) {
	for index < len(data) {
		size := int(data[index])
		index++
		if size == 0 {
			return index, nil
		}
		if size > len(data)-index {
			return 0, io.ErrUnexpectedEOF
		}
		index += size
	}
	return 0, io.ErrUnexpectedEOF
}

func targetDimensions(target busylib.DisplayTarget) (int, int, error) {
	switch target {
	case busylib.DisplayFront:
		return framepkg.FrontWidth, framepkg.FrontHeight, nil
	case busylib.DisplayBack:
		return framepkg.BackWidth, framepkg.BackHeight, nil
	default:
		return 0, 0, ErrInvalidTarget
	}
}

func resizeAndCrop(source image.Image, maximumWidth, maximumHeight int) (*image.NRGBA, error) {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, ErrInvalidImage
	}
	scale := math.Max(float64(maximumWidth)/float64(width), float64(maximumHeight)/float64(height))
	if scale > 1 {
		scale = 1
	}
	scaledWidth := max(1, int(math.Round(float64(width)*scale)))
	scaledHeight := max(1, int(math.Round(float64(height)*scale)))
	scaled := bilinearScale(source, scaledWidth, scaledHeight)

	cropWidth := min(scaledWidth, maximumWidth)
	cropHeight := min(scaledHeight, maximumHeight)
	startX := (scaledWidth - cropWidth) / 2
	startY := (scaledHeight - cropHeight) / 2
	result := image.NewNRGBA(image.Rect(0, 0, cropWidth, cropHeight))
	for y := 0; y < cropHeight; y++ {
		for x := 0; x < cropWidth; x++ {
			result.SetNRGBA(x, y, scaled.NRGBAAt(startX+x, startY+y))
		}
	}
	return result, nil
}

func bilinearScale(source image.Image, width, height int) *image.NRGBA {
	bounds := source.Bounds()
	sourceWidth, sourceHeight := bounds.Dx(), bounds.Dy()
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		sourceY := (float64(y)+0.5)*float64(sourceHeight)/float64(height) - 0.5
		y0 := clamp(int(math.Floor(sourceY)), 0, sourceHeight-1)
		y1 := clamp(y0+1, 0, sourceHeight-1)
		yWeight := sourceY - math.Floor(sourceY)
		for x := 0; x < width; x++ {
			sourceX := (float64(x)+0.5)*float64(sourceWidth)/float64(width) - 0.5
			x0 := clamp(int(math.Floor(sourceX)), 0, sourceWidth-1)
			x1 := clamp(x0+1, 0, sourceWidth-1)
			xWeight := sourceX - math.Floor(sourceX)
			c00 := color.NRGBAModel.Convert(source.At(bounds.Min.X+x0, bounds.Min.Y+y0)).(color.NRGBA)
			c10 := color.NRGBAModel.Convert(source.At(bounds.Min.X+x1, bounds.Min.Y+y0)).(color.NRGBA)
			c01 := color.NRGBAModel.Convert(source.At(bounds.Min.X+x0, bounds.Min.Y+y1)).(color.NRGBA)
			c11 := color.NRGBAModel.Convert(source.At(bounds.Min.X+x1, bounds.Min.Y+y1)).(color.NRGBA)
			result.SetNRGBA(x, y, interpolate(c00, c10, c01, c11, xWeight, yWeight))
		}
	}
	return result
}

func interpolate(c00, c10, c01, c11 color.NRGBA, xWeight, yWeight float64) color.NRGBA {
	channel := func(v00, v10, v01, v11 uint8) uint8 {
		top := float64(v00)*(1-xWeight) + float64(v10)*xWeight
		bottom := float64(v01)*(1-xWeight) + float64(v11)*xWeight
		return uint8(math.Round(top*(1-yWeight) + bottom*yWeight))
	}
	return color.NRGBA{
		R: channel(c00.R, c10.R, c01.R, c11.R),
		G: channel(c00.G, c10.G, c01.G, c11.G),
		B: channel(c00.B, c10.B, c01.B, c11.B),
		A: channel(c00.A, c10.A, c01.A, c11.A),
	}
}

func clamp(value, minimum, maximum int) int {
	return min(max(value, minimum), maximum)
}
