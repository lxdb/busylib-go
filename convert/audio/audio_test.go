package audio

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestConvertPassesThroughReadyPCMWithoutFFmpeg(t *testing.T) {
	input := []byte{1, 2, 3, 4}
	called := false
	result, err := Convert(context.Background(), bytes.NewReader(input), "ready.snd", withCommandFactory(func(context.Context, string, ...string) *exec.Cmd {
		called = true
		return nil
	}))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if called {
		t.Fatal("ffmpeg was called for ready PCM")
	}
	if result.Extension != OutputExtension || !bytes.Equal(result.Data, input) {
		t.Fatalf("result = %#v", result)
	}
	input[0] = 9
	if result.Data[0] != 1 {
		t.Fatal("result aliases caller input")
	}
}

func TestConvertInvokesConfiguredFFmpegForSupportedAudio(t *testing.T) {
	var gotPath string
	var gotArgs []string
	factory := func(ctx context.Context, path string, args ...string) *exec.Cmd {
		gotPath = path
		gotArgs = append([]string(nil), args...)
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=TestFFmpegHelperProcess", "--")
		command.Env = append(os.Environ(), "GO_WANT_FFMPEG_HELPER=success")
		return command
	}
	result, err := Convert(
		context.Background(),
		bytes.NewBufferString("compressed audio"),
		"clip.mp3",
		WithFFmpegPath("/opt/ffmpeg"),
		withCommandFactory(factory),
	)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if gotPath != "/opt/ffmpeg" {
		t.Fatalf("ffmpeg path = %q", gotPath)
	}
	wantArgs := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-i", "pipe:0", "-f", "s16le", "-acodec", "pcm_s16le", "-ac", "1", "-ar", "44100", "pipe:1"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("ffmpeg args = %#v, want %#v", gotArgs, wantArgs)
	}
	if !bytes.Equal(result.Data, []byte{1, 0, 2, 0}) {
		t.Fatalf("PCM output = %v", result.Data)
	}
}

func TestConvertSurfacesFFmpegFailure(t *testing.T) {
	factory := func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=TestFFmpegHelperProcess", "--")
		command.Env = append(os.Environ(), "GO_WANT_FFMPEG_HELPER=failure")
		return command
	}
	_, err := Convert(context.Background(), bytes.NewBufferString("audio"), "clip.wav", withCommandFactory(factory))
	if !errors.Is(err, ErrToolFailed) {
		t.Fatalf("Convert error = %v", err)
	}
	var conversionError *ConversionError
	if !errors.As(err, &conversionError) || !strings.Contains(conversionError.Stderr, "decoder failed") {
		t.Fatalf("conversion error = %#v", err)
	}
}

func TestConvertRejectsMalformedPCMAndUnsupportedInputs(t *testing.T) {
	if _, err := Convert(context.Background(), nil, "ready.snd"); !errors.Is(err, ErrInvalidAudio) {
		t.Fatalf("nil source error = %v", err)
	}
	for _, test := range []struct {
		name string
		data string
		want error
	}{
		{name: "empty.pcm", want: ErrInvalidAudio},
		{name: "odd.raw", data: "x", want: ErrInvalidAudio},
		{name: "video.mp4", data: "video", want: ErrUnsupportedFormat},
		{name: "animation.gif", data: "gif", want: ErrUnsupportedFormat},
		{name: "unknown.bin", data: "data", want: ErrUnsupportedFormat},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Convert(context.Background(), strings.NewReader(test.data), test.name)
			if !errors.Is(err, test.want) {
				t.Fatalf("Convert error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestWithFFmpegPathRejectsEmptyPath(t *testing.T) {
	if _, err := Convert(context.Background(), strings.NewReader("audio"), "clip.mp3", WithFFmpegPath("")); err == nil {
		t.Fatal("Convert accepted an empty ffmpeg path")
	}
}

func TestFFmpegHelperProcess(t *testing.T) {
	mode := os.Getenv("GO_WANT_FFMPEG_HELPER")
	if mode == "" {
		return
	}
	_, _ = os.ReadFile(os.DevNull)
	if mode == "failure" {
		_, _ = os.Stderr.WriteString("decoder failed")
		os.Exit(2)
	}
	_, _ = os.Stdout.Write([]byte{1, 0, 2, 0})
	os.Exit(0)
}
