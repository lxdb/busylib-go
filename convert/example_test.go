package convert_test

import (
	"bytes"
	"log"

	busylib "github.com/lxdb/busylib-go"
	"github.com/lxdb/busylib-go/convert"
)

func ExampleImage() {
	encodedPNG := []byte("replace with PNG data")
	result, err := convert.Image(
		bytes.NewReader(encodedPNG),
		busylib.DisplayFront,
		convert.WithMaxInputBytes(4<<20),
	)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("prepared %dx%d %s", result.Width, result.Height, result.Format)
}
