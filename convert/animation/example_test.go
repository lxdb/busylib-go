package animation_test

import (
	"image"
	"image/color"
	"log"
	"os"

	"github.com/lxdb/busylib-go/convert/animation"
)

func ExampleEncodeRGB888() {
	result, err := animation.EncodeRGB888(
		[]animation.RGB888Frame{{PixelsBGR: []byte{0x33, 0x22, 0x11}}},
		animation.RGB888Config{Width: 1, Height: 1},
	)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("prepared %dx%d animation with %d display frame", result.Width, result.Height, result.DisplayFrameCount)
}

func ExampleEncodeImages() {
	frame := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	frame.SetNRGBA(0, 0, color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff})
	result, err := animation.EncodeImages([]animation.ImageFrame{{Image: frame}}, 30)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("prepared %d animation bytes", len(result.Data))
}

func ExampleConvertZIP() {
	file, err := os.Open("spinner.zip")
	if err != nil {
		log.Print(err)
		return
	}
	defer file.Close()

	result, err := animation.ConvertZIP(file, "spinner.zip")
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("prepared %d animation bytes", len(result.Data))
}
