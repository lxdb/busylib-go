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

// Commands exposes the curated production CLI commands registered by the F21
// firmware. Arguments are passed through after newline and NUL validation.
type Commands struct {
	send   sendCommandFunc
	stream streamCommandFunc
	reboot func(context.Context) error
}

func (c Commands) Uptime(ctx context.Context) (Response, error) {
	return c.send(ctx, "uptime")
}

func (c Commands) Power(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "power", args...)
}

func (c Commands) Storage(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "storage", args...)
}

func (c Commands) Update(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "update", args...)
}

func (c Commands) Input(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "input", args...)
}

func (c Commands) Loader(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "loader", args...)
}

func (c Commands) Top(ctx context.Context, dst io.Writer, interval time.Duration) error {
	if interval < 0 {
		return errors.New("top interval must not be negative")
	}
	return c.stream(ctx, dst, "top", strconv.FormatInt(interval.Milliseconds(), 10))
}

func (c Commands) Free(ctx context.Context) (Response, error) {
	return c.send(ctx, "free")
}

func (c Commands) FreeBlocks(ctx context.Context) (Response, error) {
	return c.send(ctx, "free_blocks")
}

func (c Commands) Log(ctx context.Context, dst io.Writer, args ...string) error {
	return c.stream(ctx, dst, "log", args...)
}

func (c Commands) Echo(ctx context.Context, message string) (Response, error) {
	return c.send(ctx, "echo", message)
}

func (c Commands) DeviceInfo(ctx context.Context) (Response, error) {
	return c.send(ctx, "device_info")
}

func (c Commands) Date(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "date", args...)
}

func (c Commands) Timezone(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "timezone", args...)
}

func (c Commands) Matter(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "matter", args...)
}

func (c Commands) Audio(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "audio", args...)
}

func (c Commands) Display(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "display", args...)
}

func (c Commands) Sysctl(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "sysctl", args...)
}

func (c Commands) LogDump(ctx context.Context, args ...string) (Response, error) {
	return c.send(ctx, "log_dump", args...)
}

// Reboot sends the firmware's software-reboot command and deliberately does
// not wait for another prompt because the device is restarting.
func (c Commands) Reboot(ctx context.Context) error {
	return c.reboot(ctx)
}
