package usb

import (
	"context"
	"errors"
	"io"
	"strconv"
	"time"
)

type sendCommandFunc func(context.Context, string, ...string) (Response, error)
type streamCommandFunc func(context.Context, io.Writer, string, ...string) error

// Commands exposes the curated production CLI commands registered by the F22
// firmware. Arguments are passed through after newline and NUL validation.
type Commands struct {
	send   sendCommandFunc
	stream streamCommandFunc
	reboot func(context.Context) error
}

// Uptime returns the device uptime from the firmware CLI.
func (c Commands) Uptime(ctx context.Context) (Response, error) {
	return c.send(ctx, "uptime")
}

// Power runs the firmware power command with the supplied arguments.
func (c Commands) Power(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "power", args...)
}

// Storage runs the firmware storage command with the supplied arguments.
func (c Commands) Storage(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "storage", args...)
}

// Update runs the firmware update command with the supplied arguments.
func (c Commands) Update(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "update", args...)
}

// Input runs the firmware input command with the supplied arguments.
func (c Commands) Input(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "input", args...)
}

// Loader runs the firmware loader command with the supplied arguments.
func (c Commands) Loader(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "loader", args...)
}

// Top writes process statistics to dst until the context ends or the prompt returns.
// It reports an error when interval is negative.
func (c Commands) Top(ctx context.Context, dst io.Writer, interval time.Duration) error {
	if interval < 0 {
		return errors.New("top interval must not be negative")
	}
	return c.stream(ctx, dst, "top", strconv.FormatInt(interval.Milliseconds(), 10))
}

// Free returns the firmware memory summary.
func (c Commands) Free(ctx context.Context) (Response, error) {
	return c.send(ctx, "free")
}

// FreeBlocks returns the firmware free-block summary.
func (c Commands) FreeBlocks(ctx context.Context) (Response, error) {
	return c.send(ctx, "free_blocks")
}

// Log writes firmware log output to dst until the context ends or the prompt returns.
func (c Commands) Log(ctx context.Context, dst io.Writer, args ...string) error {
	return c.stream(ctx, dst, "log", args...)
}

// Echo returns the firmware response to message.
func (c Commands) Echo(ctx context.Context, message string) (Response, error) {
	return c.send(ctx, "echo", message)
}

// DeviceInfo returns the firmware device information summary.
func (c Commands) DeviceInfo(ctx context.Context) (Response, error) {
	return c.send(ctx, "device_info")
}

// Date runs the firmware date command with the supplied arguments.
func (c Commands) Date(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "date", args...)
}

// Timezone runs the firmware timezone command with the supplied arguments.
func (c Commands) Timezone(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "timezone", args...)
}

// Matter runs the firmware Matter command with the supplied arguments.
func (c Commands) Matter(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "matter", args...)
}

// Audio runs the firmware audio command with the supplied arguments.
func (c Commands) Audio(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "audio", args...)
}

// Display runs the firmware display command with the supplied arguments.
func (c Commands) Display(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "display", args...)
}

// Sysctl runs the firmware system-control command with the supplied arguments.
func (c Commands) Sysctl(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "sysctl", args...)
}

// LogDump runs the bounded firmware log-dump command with the supplied arguments.
func (c Commands) LogDump(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "log_dump", args...)
}

// Reboot sends the firmware's software-reboot command and deliberately does
// not wait for another prompt because the device is restarting.
func (c Commands) Reboot(ctx context.Context) error {
	return c.reboot(ctx)
}
