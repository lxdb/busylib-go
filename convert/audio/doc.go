// Package audio prepares headerless mono PCM audio for BUSY Bar playback.
//
// Convert passes through device-ready .snd, .raw, and .pcm input. It invokes
// ffmpeg for supported encoded formats. The ffmpeg executable is an optional
// runtime tool, not a Go module dependency.
package audio
