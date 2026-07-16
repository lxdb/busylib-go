package audio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	OutputExtension = ".snd"
	Channels        = 1
	SampleRateHz    = 44_100
	BitsPerSample   = 16
	maxToolStderr   = 4096
)

var (
	readyExtensions = map[string]struct{}{OutputExtension: {}, ".raw": {}, ".pcm": {}}
	inputExtensions = map[string]struct{}{".mp3": {}, ".ogg": {}, ".aac": {}, ".m4a": {}, ".flac": {}, ".wav": {}}
)

type commandFactory func(context.Context, string, ...string) *exec.Cmd

type config struct {
	ffmpegPath     string
	commandFactory commandFactory
}

// Option configures audio conversion.
type Option func(*config) error

// WithFFmpegPath selects the ffmpeg executable used for compressed inputs.
func WithFFmpegPath(path string) Option {
	return func(config *config) error {
		if strings.TrimSpace(path) == "" {
			return errors.New("ffmpeg path must not be empty")
		}
		config.ffmpegPath = path
		return nil
	}
}

func withCommandFactory(factory commandFactory) Option {
	return func(config *config) error {
		if factory == nil {
			return errors.New("audio command factory must not be nil")
		}
		config.commandFactory = factory
		return nil
	}
}

// Result owns raw headerless mono 44.1 kHz signed 16-bit little-endian PCM.
type Result struct {
	Data      []byte
	Extension string
}

// Convert passes through already-ready PCM or invokes ffmpeg for supported
// audio inputs. The filename extension selects the input behavior.
func Convert(ctx context.Context, source io.Reader, filename string, options ...Option) (Result, error) {
	config := config{ffmpegPath: "ffmpeg", commandFactory: exec.CommandContext}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&config); err != nil {
			return Result{}, err
		}
	}
	extension := strings.ToLower(filepath.Ext(filename))
	if source == nil {
		return Result{}, &ConversionError{Operation: "validate", InputFormat: extension, Err: ErrInvalidAudio}
	}
	if _, ok := readyExtensions[extension]; ok {
		return readReadyPCM(source, extension)
	}
	if _, ok := inputExtensions[extension]; !ok {
		return Result{}, &ConversionError{Operation: "validate", InputFormat: extension, Err: ErrUnsupportedFormat}
	}
	return runFFmpeg(ctx, source, extension, config)
}

// ConvertFile converts the contents of path using its extension.
func ConvertFile(ctx context.Context, path string, options ...Option) (Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return Result{}, &ConversionError{Operation: "open", InputFormat: strings.ToLower(filepath.Ext(path)), Err: err}
	}
	defer file.Close()
	return Convert(ctx, file, path, options...)
}

func readReadyPCM(source io.Reader, extension string) (Result, error) {
	data, err := io.ReadAll(source)
	if err != nil {
		return Result{}, &ConversionError{Operation: "read", InputFormat: extension, Err: err}
	}
	if len(data) == 0 || len(data)%2 != 0 {
		return Result{}, &ConversionError{Operation: "validate", InputFormat: extension, Err: ErrInvalidAudio}
	}
	return Result{Data: append([]byte(nil), data...), Extension: OutputExtension}, nil
}

func runFFmpeg(ctx context.Context, source io.Reader, extension string, config config) (Result, error) {
	arguments := []string{
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-i", "pipe:0",
		"-f", "s16le", "-acodec", "pcm_s16le", "-ac", "1", "-ar", "44100",
		"pipe:1",
	}
	command := config.commandFactory(ctx, config.ffmpegPath, arguments...)
	command.Stdin = source
	var output bytes.Buffer
	stderr := &limitedBuffer{maximum: maxToolStderr}
	command.Stdout = &output
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		cause := errors.Join(ErrToolFailed, err)
		if ctx.Err() != nil {
			cause = errors.Join(ErrToolFailed, ctx.Err())
		}
		return Result{}, &ConversionError{
			Operation:   "transcode",
			InputFormat: extension,
			Tool:        config.ffmpegPath,
			Stderr:      strings.TrimSpace(stderr.String()),
			Err:         cause,
		}
	}
	if output.Len() == 0 || output.Len()%2 != 0 {
		return Result{}, &ConversionError{
			Operation:   "validate",
			InputFormat: extension,
			Tool:        config.ffmpegPath,
			Err:         ErrInvalidAudio,
		}
	}
	return Result{Data: append([]byte(nil), output.Bytes()...), Extension: OutputExtension}, nil
}

type limitedBuffer struct {
	bytes.Buffer
	maximum int
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	count := len(data)
	remaining := b.maximum - b.Len()
	if remaining > 0 {
		_, _ = b.Buffer.Write(data[:min(len(data), remaining)])
	}
	return count, nil
}
