package animation

import (
	"archive/zip"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testZIPEntry struct {
	name string
	data []byte
}

func TestConvertZIPMatchesRawEncoder(t *testing.T) {
	archive := makeTestZIP(t,
		testZIPEntry{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"rgb888","sections":[]}`)},
		testZIPEntry{name: "sample/ignored.txt", data: []byte("ignored")},
		testZIPEntry{name: "sample/frame_0.png", data: encodeTestPNG(t, image.Rect(0, 0, 1, 1), color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff})},
	)

	fromZIP, err := ConvertZIP(bytes.NewReader(archive), "sample.zip")
	if err != nil {
		t.Fatalf("ConvertZIP: %v", err)
	}
	fromRaw, err := EncodeRGB888(
		[]RGB888Frame{{PixelsBGR: []byte{0x33, 0x22, 0x11}}},
		RGB888Config{Width: 1, Height: 1, FPS: 30},
	)
	if err != nil {
		t.Fatalf("EncodeRGB888: %v", err)
	}
	if !bytes.Equal(fromZIP.Data, fromRaw.Data) {
		t.Fatalf("ZIP and raw encoders differ:\nZIP %x\nraw %x", fromZIP.Data, fromRaw.Data)
	}
}

func TestConvertZIPFileReadsFileAndPreservesOpenErrors(t *testing.T) {
	archive := makeTestZIP(t,
		testZIPEntry{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"rgb888","sections":[]}`)},
		testZIPEntry{name: "sample/frame_0.png", data: encodeTestPNG(t, image.Rect(0, 0, 1, 1), color.NRGBA{A: 0xff})},
	)
	path := filepath.Join(t.TempDir(), "sample.zip")
	if err := os.WriteFile(path, archive, 0o600); err != nil {
		t.Fatalf("write ZIP fixture: %v", err)
	}

	fromFile, err := ConvertZIPFile(path)
	if err != nil {
		t.Fatalf("ConvertZIPFile: %v", err)
	}
	fromReader, err := ConvertZIP(bytes.NewReader(archive), "sample.zip")
	if err != nil {
		t.Fatalf("ConvertZIP: %v", err)
	}
	if !bytes.Equal(fromFile.Data, fromReader.Data) {
		t.Fatal("file and reader conversions differ")
	}

	_, err = ConvertZIPFile(filepath.Join(t.TempDir(), "missing.zip"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file error = %v, want fs.ErrNotExist", err)
	}
	var conversionErr *ConversionError
	if !errors.As(err, &conversionErr) || conversionErr.Operation != "open" {
		t.Fatalf("missing file error = %#v, want open ConversionError", conversionErr)
	}
}

func TestConvertZIPRejectsInvalidMetadataAndFrames(t *testing.T) {
	validPNG := encodeTestPNG(t, image.Rect(0, 0, 1, 1), color.NRGBA{A: 0xff})
	tests := []struct {
		name     string
		filename string
		entries  []testZIPEntry
		want     error
	}{
		{
			name:     "missing metadata",
			filename: "sample.zip",
			entries:  []testZIPEntry{{name: "sample/frame_0.png", data: validPNG}},
			want:     ErrInvalidMetadata,
		},
		{
			name:     "duplicate metadata",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"rgb888","sections":[]}`)},
				{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"rgb888","sections":[]}`)},
				{name: "sample/frame_0.png", data: validPNG},
			},
			want: ErrInvalidMetadata,
		},
		{
			name:     "missing sections",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"rgb888"}`)},
				{name: "sample/frame_0.png", data: validPNG},
			},
			want: ErrInvalidMetadata,
		},
		{
			name:     "malformed metadata",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"fps":`)},
				{name: "sample/frame_0.png", data: validPNG},
			},
			want: ErrInvalidMetadata,
		},
		{
			name:     "missing fps",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"color_mode":"rgb888","sections":[]}`)},
				{name: "sample/frame_0.png", data: validPNG},
			},
			want: ErrInvalidMetadata,
		},
		{
			name:     "invalid fps",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"fps":256,"color_mode":"rgb888","sections":[]}`)},
				{name: "sample/frame_0.png", data: validPNG},
			},
			want: ErrInvalidMetadata,
		},
		{
			name:     "missing color mode",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"fps":30,"sections":[]}`)},
				{name: "sample/frame_0.png", data: validPNG},
			},
			want: ErrInvalidMetadata,
		},
		{
			name:     "unsupported color mode",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"gray4","sections":[]}`)},
				{name: "sample/frame_0.png", data: validPNG},
			},
			want: ErrUnsupportedColorMode,
		},
		{
			name:     "custom sections",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"rgb888","sections":[{"name":"blink"}]}`)},
				{name: "sample/frame_0.png", data: validPNG},
			},
			want: ErrUnsupportedSections,
		},
		{
			name:     "null sections",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"rgb888","sections":null}`)},
				{name: "sample/frame_0.png", data: validPNG},
			},
			want: ErrInvalidMetadata,
		},
		{
			name:     "no frames",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"rgb888","sections":[]}`)},
			},
			want: ErrNoFrames,
		},
		{
			name:     "missing frame zero",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"rgb888","sections":[]}`)},
				{name: "sample/frame_1.png", data: validPNG},
			},
			want: ErrInvalidFrame,
		},
		{
			name:     "mismatched dimensions",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"rgb888","sections":[]}`)},
				{name: "sample/frame_0.png", data: validPNG},
				{name: "sample/frame_1.png", data: encodeTestPNG(t, image.Rect(0, 0, 2, 1), color.NRGBA{A: 0xff})},
			},
			want: ErrInvalidFrame,
		},
		{
			name:     "duplicate frame index",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"rgb888","sections":[]}`)},
				{name: "sample/frame_0.png", data: validPNG},
				{name: "sample/frame_00.png", data: validPNG},
			},
			want: ErrInvalidFrame,
		},
		{
			name:     "malformed PNG",
			filename: "sample.zip",
			entries: []testZIPEntry{
				{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"rgb888","sections":[]}`)},
				{name: "sample/frame_0.png", data: []byte("not a PNG")},
			},
			want: ErrInvalidFrame,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archive := makeTestZIP(t, test.entries...)
			_, err := ConvertZIP(bytes.NewReader(archive), test.filename)
			if !errors.Is(err, test.want) {
				t.Fatalf("ConvertZIP error = %v, want errors.Is(_, %v)", err, test.want)
			}
			var conversionErr *ConversionError
			if !errors.As(err, &conversionErr) || conversionErr.Operation == "" {
				t.Fatalf("ConvertZIP error = %#v, want contextual *ConversionError", conversionErr)
			}
		})
	}
}

func TestConvertZIPSortsFramesAndCoalescesDuplicates(t *testing.T) {
	frame := encodeTestPNG(t, image.Rect(0, 0, 1, 1), color.NRGBA{R: 1, G: 2, B: 3, A: 0xff})
	archive := makeTestZIP(t,
		testZIPEntry{name: "sample/meta.json", data: []byte(`{"fps":60,"color_mode":"rgb888","sections":[]}`)},
		testZIPEntry{name: "sample/frame_1.png", data: frame},
		testZIPEntry{name: "sample/frame_0.png", data: frame},
	)
	result, err := ConvertZIP(bytes.NewReader(archive), "sample.zip")
	if err != nil {
		t.Fatalf("ConvertZIP: %v", err)
	}
	if result.FPS != 60 || result.EncodedFrameCount != 1 || result.DisplayFrameCount != 2 {
		t.Fatalf("result = %#v, want 60 FPS with 1 encoded/2 display frames", result)
	}
}

func TestConvertZIPEnforcesInputExpandedAndOutputLimits(t *testing.T) {
	validMeta := []byte(`{"fps":30,"color_mode":"rgb888","sections":[]}`)
	validPNG := encodeTestPNG(t, image.Rect(0, 0, 1, 1), color.NRGBA{A: 0xff})
	archive := makeTestZIP(t,
		testZIPEntry{name: "sample/meta.json", data: validMeta},
		testZIPEntry{name: "sample/frame_0.png", data: validPNG},
	)

	_, err := ConvertZIP(bytes.NewReader(archive), "sample.zip", WithMaxInputBytes(int64(len(archive)-1)))
	if !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("compressed input limit error = %v, want ErrInputTooLarge", err)
	}
	_, err = ConvertZIP(bytes.NewReader(archive), "sample.zip", WithMaxOutputBytes(63))
	if !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("output limit error = %v, want ErrOutputTooLarge", err)
	}

	expandedMeta := append(append([]byte(nil), validMeta...), []byte(strings.Repeat(" ", 4096))...)
	expandedArchive := makeTestZIP(t,
		testZIPEntry{name: "sample/meta.json", data: expandedMeta},
		testZIPEntry{name: "sample/frame_0.png", data: validPNG},
	)
	expandedLimit := int64(len(expandedArchive) + 1)
	if expandedLimit >= int64(len(expandedMeta)+len(validPNG)) {
		t.Fatalf("expanded-limit fixture is not compressed enough: archive=%d expanded=%d", expandedLimit, len(expandedMeta)+len(validPNG))
	}
	_, err = ConvertZIP(bytes.NewReader(expandedArchive), "sample.zip", WithMaxInputBytes(expandedLimit))
	if !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("expanded input limit error = %v, want ErrInputTooLarge", err)
	}
}

func TestConvertZIPRejectsMalformedArchiveAndFilename(t *testing.T) {
	for _, test := range []struct {
		name     string
		data     []byte
		filename string
	}{
		{name: "malformed archive", data: []byte("not a ZIP"), filename: "sample.zip"},
		{name: "wrong extension", data: []byte("not a ZIP"), filename: "sample.anim"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := ConvertZIP(bytes.NewReader(test.data), test.filename)
			if !errors.Is(err, ErrInvalidMetadata) {
				t.Fatalf("ConvertZIP error = %v, want ErrInvalidMetadata", err)
			}
		})
	}
}

func FuzzConvertZIP(f *testing.F) {
	valid := makeTestZIPForFuzz(f,
		testZIPEntry{name: "sample/meta.json", data: []byte(`{"fps":30,"color_mode":"rgb888","sections":[]}`)},
		testZIPEntry{name: "sample/frame_0.png", data: encodeTestPNGForFuzz(f)},
	)
	f.Add(valid)
	f.Add([]byte{})
	f.Add([]byte("not a ZIP"))

	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := ConvertZIP(
			bytes.NewReader(data),
			"sample.zip",
			WithMaxInputBytes(1<<20),
			WithMaxOutputBytes(1<<20),
		)
		if err == nil {
			if len(result.Data) < len(signature) || string(result.Data[:len(signature)]) != signature {
				t.Fatalf("accepted result does not begin with %q", signature)
			}
			if result.EncodedFrameCount <= 0 || result.DisplayFrameCount <= 0 {
				t.Fatalf("accepted result has invalid frame counts: %#v", result)
			}
		}
	})
}

func makeTestZIP(t *testing.T, entries ...testZIPEntry) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, entry := range entries {
		file, err := writer.Create(entry.name)
		if err != nil {
			t.Fatalf("create ZIP entry %s: %v", entry.name, err)
		}
		if _, err := file.Write(entry.data); err != nil {
			t.Fatalf("write ZIP entry %s: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return output.Bytes()
}

func encodeTestPNG(t *testing.T, bounds image.Rectangle, pixel color.NRGBA) []byte {
	t.Helper()
	frame := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			frame.SetNRGBA(x, y, pixel)
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, frame); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return output.Bytes()
}

func makeTestZIPForFuzz(f *testing.F, entries ...testZIPEntry) []byte {
	f.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, entry := range entries {
		file, err := writer.Create(entry.name)
		if err != nil {
			f.Fatalf("create ZIP entry %s: %v", entry.name, err)
		}
		if _, err := file.Write(entry.data); err != nil {
			f.Fatalf("write ZIP entry %s: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		f.Fatalf("close ZIP: %v", err)
	}
	return output.Bytes()
}

func encodeTestPNGForFuzz(f *testing.F) []byte {
	f.Helper()
	frame := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	frame.SetNRGBA(0, 0, color.NRGBA{R: 1, G: 2, B: 3, A: 0xff})
	var output bytes.Buffer
	if err := png.Encode(&output, frame); err != nil {
		f.Fatalf("encode PNG: %v", err)
	}
	return output.Bytes()
}
