package frame_test

import (
	"image"
	"log"

	"github.com/lxdb/busylib-go/frame"
)

func ExampleFromHTTP() {
	raw := make([]byte, frame.FrontWidth*frame.FrontHeight*3)
	value, err := frame.FromHTTP(0, raw)
	if err != nil {
		log.Print(err)
		return
	}
	var decoded image.Image
	decoded, err = value.RGBA()
	if err != nil {
		log.Print(err)
		return
	}
	_ = decoded
}
