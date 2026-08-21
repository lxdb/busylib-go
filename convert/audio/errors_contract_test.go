package audio_test

import (
	"errors"
	"testing"

	"github.com/lxdb/busylib-go/convert/audio"
)

func TestConversionErrorContract(t *testing.T) {
	cause := errors.New("exit status 1")
	tests := []struct {
		err  *audio.ConversionError
		want string
	}{
		{&audio.ConversionError{Operation: "transcode", InputFormat: ".mp3", Tool: "ffmpeg", Stderr: "bad data", Err: cause}, "audio transcode failed for .mp3 input using ffmpeg: exit status 1: bad data"},
		{&audio.ConversionError{Operation: "read", Err: cause}, "audio read failed: exit status 1"},
	}
	for _, test := range tests {
		if got := test.err.Error(); got != test.want {
			t.Fatalf("Error() = %q, want %q", got, test.want)
		}
		if !errors.Is(test.err, cause) {
			t.Fatal("ConversionError did not preserve its cause")
		}
	}
}
