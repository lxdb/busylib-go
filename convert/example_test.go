package convert_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"

	busylib "github.com/lxdb/busylib-go"
	"github.com/lxdb/busylib-go/convert"
)

func ExampleImage() {
	input := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	input.SetNRGBA(0, 0, color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff})
	var encodedPNG bytes.Buffer
	if err := png.Encode(&encodedPNG, input); err != nil {
		return
	}
	result, err := convert.Image(
		bytes.NewReader(encodedPNG.Bytes()),
		busylib.DisplayFront,
		convert.WithMaxInputBytes(4<<20),
	)
	if err != nil {
		return
	}
	fmt.Printf("prepared %dx%d %s\n", result.Width, result.Height, result.Format)
	// Output:
	// prepared 1x1 png
}
