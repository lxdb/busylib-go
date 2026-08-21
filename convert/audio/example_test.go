package audio_test

import (
	"bytes"
	"context"
	"log"

	"github.com/lxdb/busylib-go/convert/audio"
)

func ExampleConvert() {
	pcm := []byte{0, 0, 1, 0}
	result, err := audio.Convert(context.Background(), bytes.NewReader(pcm), "tone.snd")
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("prepared %d bytes", len(result.Data))
}
