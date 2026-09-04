package usb

import (
	"errors"
	"strings"
	"time"
)

const (
	// DefaultAddress is the firmware CLI address on the USB network interface.
	DefaultAddress = "10.0.4.20:23"
	prompt         = ">: "
	interruptByte  = byte(3)

	defaultDialTimeout      = 2 * time.Second
	defaultCommandTimeout   = 5 * time.Second
	defaultMaxResponseBytes = 1 << 20
)

type config struct {
	address          string
	dialTimeout      time.Duration
	commandTimeout   time.Duration
	maxResponseBytes int
}

// Option configures a USB CLI client. NewClient ignores nil options.
type Option func(*config) error

// WithAddress changes the raw CLI TCP address in host:port form.
func WithAddress(address string) Option {
	return func(config *config) error {
		if strings.TrimSpace(address) == "" {
			return errors.New("USB CLI address must not be empty")
		}
		config.address = address
		return nil
	}
}

// WithDialTimeout changes the maximum time allowed to establish a connection.
func WithDialTimeout(timeout time.Duration) Option {
	return func(config *config) error {
		if timeout <= 0 {
			return errors.New("USB CLI dial timeout must be greater than zero")
		}
		config.dialTimeout = timeout
		return nil
	}
}

// WithCommandTimeout changes the maximum time a bounded command or prompt
// recovery may take. Continuous commands are bounded by their context.
func WithCommandTimeout(timeout time.Duration) Option {
	return func(config *config) error {
		if timeout <= 0 {
			return errors.New("USB CLI command timeout must be greater than zero")
		}
		config.commandTimeout = timeout
		return nil
	}
}

// WithMaxResponseBytes sets the positive limit for buffered command responses.
func WithMaxResponseBytes(maximum int) Option {
	return func(config *config) error {
		if maximum <= 0 {
			return errors.New("USB CLI maximum response size must be greater than zero")
		}
		config.maxResponseBytes = maximum
		return nil
	}
}

func defaultConfig() config {
	return config{
		address:          DefaultAddress,
		dialTimeout:      defaultDialTimeout,
		commandTimeout:   defaultCommandTimeout,
		maxResponseBytes: defaultMaxResponseBytes,
	}
}
