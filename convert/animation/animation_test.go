package animation

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"testing"
)

func TestEncodeRGB888MatchesBicycle0GoldenFile(t *testing.T) {
	want, err := hex.DecodeString(
		"62696379636c6530" +
			"000101001e030000" +
			"15000000" +
			"07000000" +
			"01000000" +
			"01000000" +
			"01000000" +
			"00000000" +
			"00000000" +
			"39000000" +
			"01" +
			"64656661756c7400" +
			"00010300" +
			"332211",
	)
	if err != nil {
		t.Fatalf("decode golden animation: %v", err)
	}

	result, err := EncodeRGB888(
		[]RGB888Frame{{PixelsBGR: []byte{0x33, 0x22, 0x11}, Duration: 1}},
		RGB888Config{Width: 1, Height: 1, FPS: 30},
	)
	if err != nil {
		t.Fatalf("EncodeRGB888: %v", err)
	}
	if got := hex.EncodeToString(result.Data); got != hex.EncodeToString(want) {
		t.Fatalf("encoded animation = %s, want %s", got, hex.EncodeToString(want))
	}
	if result.Width != 1 || result.Height != 1 || result.FPS != 30 {
		t.Fatalf("result dimensions/FPS = %dx%d@%d, want 1x1@30", result.Width, result.Height, result.FPS)
	}
	if result.EncodedFrameCount != 1 || result.DisplayFrameCount != 1 {
		t.Fatalf("result frame counts = encoded %d/display %d, want 1/1", result.EncodedFrameCount, result.DisplayFrameCount)
	}
}

func TestCalculateLayoutRejectsUnrepresentableCounts(t *testing.T) {
	valid, err := calculateLayout(3, 1, 1, 64)
	if err != nil {
		t.Fatalf("calculateLayout valid input: %v", err)
	}
	if valid.sectionsLength != 21 || valid.framesLength != 7 || valid.outputLength != 64 {
		t.Fatalf("valid layout = %#v, want sections=21 frames=7 output=64", valid)
	}

	tests := []struct {
		name         string
		frameLength  uint64
		frameCount   uint64
		displayCount uint64
		maximum      int64
	}{
		{name: "frame chunk overflows uint32", frameLength: 3, frameCount: math.MaxUint32, displayCount: 1, maximum: math.MaxInt64},
		{name: "display count overflows uint32", frameLength: 3, frameCount: 1, displayCount: math.MaxUint32 + 1, maximum: math.MaxInt64},
		{name: "configured output limit", frameLength: 3, frameCount: 1, displayCount: 1, maximum: 63},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := calculateLayout(test.frameLength, test.frameCount, test.displayCount, test.maximum)
			if !errors.Is(err, ErrOutputTooLarge) {
				t.Fatalf("calculateLayout error = %v, want ErrOutputTooLarge", err)
			}
		})
	}
}

func TestEncodeRGB888CoalescesAndSplitsIdenticalFrames(t *testing.T) {
	pixels := []byte{0x30, 0x20, 0x10}
	result, err := EncodeRGB888(
		[]RGB888Frame{
			{PixelsBGR: pixels, Duration: 250},
			{PixelsBGR: append([]byte(nil), pixels...), Duration: 10},
		},
		RGB888Config{Width: 1, Height: 1},
	)
	if err != nil {
		t.Fatalf("EncodeRGB888: %v", err)
	}
	if result.FPS != DefaultFPS {
		t.Fatalf("FPS = %d, want default %d", result.FPS, DefaultFPS)
	}
	if result.EncodedFrameCount != 2 || result.DisplayFrameCount != 260 {
		t.Fatalf("frame counts = encoded %d/display %d, want 2/260", result.EncodedFrameCount, result.DisplayFrameCount)
	}
	if got := binary.LittleEndian.Uint32(result.Data[28:32]); got != 2 {
		t.Fatalf("header encoded frame count = %d, want 2", got)
	}
	if got := binary.LittleEndian.Uint32(result.Data[32:36]); got != 260 {
		t.Fatalf("header display frame count = %d, want 260", got)
	}

	firstFrameOffset := int(binary.LittleEndian.Uint32(result.Data[44:48]))
	if got := result.Data[firstFrameOffset+1]; got != 255 {
		t.Fatalf("first encoded duration = %d, want 255", got)
	}
	secondFrameOffset := firstFrameOffset + 4 + len(pixels)
	if got := result.Data[secondFrameOffset+1]; got != 5 {
		t.Fatalf("second encoded duration = %d, want 5", got)
	}
	if !bytes.Equal(result.Data[firstFrameOffset+4:firstFrameOffset+7], pixels) ||
		!bytes.Equal(result.Data[secondFrameOffset+4:secondFrameOffset+7], pixels) {
		t.Fatal("coalesced records changed pixel bytes")
	}
}
