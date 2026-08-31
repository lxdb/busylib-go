package audio_test

import (
	"bytes"
	"context"
	"fmt"

	"github.com/lxdb/busylib-go/convert/audio"
)

func ExampleConvert() {
	pcm := []byte{0, 0, 1, 0}
	result, err := audio.Convert(context.Background(), bytes.NewReader(pcm), "tone.snd")
	if err != nil {
		return
	}
	fmt.Printf("prepared %d bytes\n", len(result.Data))
	// Output:
	// prepared 4 bytes
}
