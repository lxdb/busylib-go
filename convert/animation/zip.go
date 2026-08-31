package animation

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ConvertZIP converts a firmware-style ZIP containing meta.json and numbered
// PNG frames into a device-native animation. It does not close source.
func ConvertZIP(source io.Reader, filename string, options ...Option) (Result, error) {
	conversionConfig, err := newConfig(options)
	if err != nil {
		return Result{}, err
	}
	if source == nil {
		return Result{}, conversionError("read", -1, "", ErrInvalidMetadata)
	}
	base := filepath.Base(filename)
	extension := filepath.Ext(base)
	if !strings.EqualFold(extension, ".zip") {
		return Result{}, conversionError("validate", -1, "", fmt.Errorf(
			"%w: filename %q does not have a .zip extension", ErrInvalidMetadata, base,
		))
	}
	root := strings.TrimSuffix(base, extension)
	if root == "" || root == "." {
		return Result{}, conversionError("validate", -1, "", ErrInvalidMetadata)
	}

	archiveData, err := readBounded(source, conversionConfig.maxInputBytes)
	if err != nil {
		return Result{}, conversionError("read", -1, "", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(archiveData), int64(len(archiveData)))
	if err != nil {
		return Result{}, conversionError("decode", -1, "", errors.Join(ErrInvalidMetadata, err))
	}

	metaName := root + "/meta.json"
	framePattern := regexp.MustCompile("^" + regexp.QuoteMeta(root) + `/frame_(\d+)\.png$`)
	var metaFile *zip.File
	framesByIndex := make(map[int]*zip.File)
	relevantBytes := uint64(0)
	for _, file := range archive.File {
		if file.Name == metaName {
			if metaFile != nil {
				return Result{}, conversionError("validate", -1, file.Name, fmt.Errorf(
					"%w: duplicate metadata entry", ErrInvalidMetadata,
				))
			}
			metaFile = file
			if err := addExpandedSize(&relevantBytes, file.UncompressedSize64, conversionConfig.maxInputBytes); err != nil {
				return Result{}, conversionError("read", -1, file.Name, err)
			}
			continue
		}
		match := framePattern.FindStringSubmatch(file.Name)
		if match == nil {
			continue
		}
		index64, err := strconv.ParseUint(match[1], 10, 31)
		if err != nil {
			return Result{}, conversionError("validate", -1, file.Name, fmt.Errorf(
				"%w: invalid frame index", ErrInvalidFrame,
			))
		}
		index := int(index64)
		if _, exists := framesByIndex[index]; exists {
			return Result{}, conversionError("validate", index, file.Name, fmt.Errorf(
				"%w: duplicate frame index", ErrInvalidFrame,
			))
		}
		framesByIndex[index] = file
		if err := addExpandedSize(&relevantBytes, file.UncompressedSize64, conversionConfig.maxInputBytes); err != nil {
			return Result{}, conversionError("read", index, file.Name, err)
		}
	}
	if metaFile == nil {
		return Result{}, conversionError("validate", -1, metaName, fmt.Errorf(
			"%w: metadata entry is missing", ErrInvalidMetadata,
		))
	}
	if len(framesByIndex) == 0 {
		return Result{}, conversionError("validate", -1, "", ErrNoFrames)
	}

	metaData, err := readZIPEntry(metaFile, conversionConfig.maxInputBytes)
	if err != nil {
		return Result{}, conversionError("read", -1, metaFile.Name, err)
	}
	fps, err := parseZIPMetadata(metaData)
	if err != nil {
		return Result{}, conversionError("validate", -1, metaFile.Name, err)
	}

	indexes := make([]int, 0, len(framesByIndex))
	for index := range framesByIndex {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	for expected, index := range indexes {
		if index != expected {
			return Result{}, conversionError("validate", index, framesByIndex[index].Name, fmt.Errorf(
				"%w: missing frame_%d.png", ErrInvalidFrame, expected,
			))
		}
	}

	var width, height int
	for _, index := range indexes {
		file := framesByIndex[index]
		frameData, err := readZIPEntry(file, conversionConfig.maxInputBytes)
		if err != nil {
			return Result{}, conversionError("read", index, file.Name, err)
		}
		imageConfig, err := png.DecodeConfig(bytes.NewReader(frameData))
		if err != nil {
			return Result{}, conversionError("decode", index, file.Name, errors.Join(ErrInvalidFrame, err))
		}
		if imageConfig.Width <= 0 || imageConfig.Height <= 0 ||
			imageConfig.Width > maxByteValue || imageConfig.Height > maxByteValue ||
			imageConfig.Width*imageConfig.Height*3 > maxFrameBytes {
			return Result{}, conversionError("validate", index, file.Name, fmt.Errorf(
				"%w: dimensions %dx%d cannot be represented as RGB888", ErrInvalidFrame, imageConfig.Width, imageConfig.Height,
			))
		}
		if index == 0 {
			width, height = imageConfig.Width, imageConfig.Height
		} else if imageConfig.Width != width || imageConfig.Height != height {
			return Result{}, conversionError("validate", index, file.Name, fmt.Errorf(
				"%w: dimensions are %dx%d, want %dx%d", ErrInvalidFrame, imageConfig.Width, imageConfig.Height, width, height,
			))
		}
	}

	return encodeFrames(
		len(indexes),
		RGB888Config{Width: width, Height: height, FPS: fps},
		conversionConfig,
		func(position int, pixels []byte) (uint8, string, error) {
			index := indexes[position]
			file := framesByIndex[index]
			frameData, err := readZIPEntry(file, conversionConfig.maxInputBytes)
			if err != nil {
				return 0, file.Name, conversionError("read", index, file.Name, err)
			}
			decoded, err := png.Decode(bytes.NewReader(frameData))
			if err != nil {
				return 0, file.Name, conversionError("decode", index, file.Name, errors.Join(ErrInvalidFrame, err))
			}
			fillImagePixels(pixels, decoded)
			return 0, file.Name, nil
		},
	)
}

// ConvertZIPFile converts the firmware-style animation ZIP at path.
func ConvertZIPFile(path string, options ...Option) (Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return Result{}, conversionError("open", -1, path, err)
	}
	defer func() { _ = file.Close() }()
	return ConvertZIP(file, filepath.Base(path), options...)
}

func parseZIPMetadata(data []byte) (int, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return 0, errors.Join(ErrInvalidMetadata, err)
	}
	var fps int
	value, ok := fields["fps"]
	if !ok || json.Unmarshal(value, &fps) != nil || fps <= 0 || fps > maxByteValue {
		return 0, fmt.Errorf("%w: fps must be an integer between 1 and %d", ErrInvalidMetadata, maxByteValue)
	}
	var colorMode string
	value, ok = fields["color_mode"]
	if !ok || json.Unmarshal(value, &colorMode) != nil {
		return 0, fmt.Errorf("%w: color_mode must be present", ErrInvalidMetadata)
	}
	if colorMode != "rgb888" {
		return 0, fmt.Errorf("%w: %q", ErrUnsupportedColorMode, colorMode)
	}
	var sections []json.RawMessage
	value, ok = fields["sections"]
	if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) || json.Unmarshal(value, &sections) != nil {
		return 0, fmt.Errorf("%w: sections must be an array", ErrInvalidMetadata)
	}
	if len(sections) != 0 {
		return 0, ErrUnsupportedSections
	}
	return fps, nil
}

func readZIPEntry(file *zip.File, maximum int64) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	return readBounded(reader, maximum)
}

func readBounded(reader io.Reader, maximum int64) ([]byte, error) {
	if maximum == math.MaxInt64 {
		data, err := io.ReadAll(reader)
		return data, err
	}
	data, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, fmt.Errorf("%w: limit is %d bytes", ErrInputTooLarge, maximum)
	}
	return data, nil
}

func addExpandedSize(total *uint64, size uint64, maximum int64) error {
	limit := uint64(maximum)
	if size > limit-*total {
		return fmt.Errorf("%w: relevant expanded entries exceed %d bytes", ErrInputTooLarge, maximum)
	}
	*total += size
	return nil
}
