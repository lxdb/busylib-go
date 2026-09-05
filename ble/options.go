package ble

import (
	"errors"
	"time"
)

const (
	defaultConnectTimeout = 15 * time.Second
	defaultRequestTimeout = 10 * time.Second
	// DefaultMaxMessageBytes is the default limit for serialized requests,
	// buffered responses, and assembled FFE1 state messages.
	DefaultMaxMessageBytes int64 = 1 << 20
)

// Option configures Connect. Nil options are ignored.
type Option func(*config) error

type config struct {
	connectTimeout  time.Duration
	requestTimeout  time.Duration
	maxMessageBytes int64
}

func defaultConfig() config {
	return config{
		connectTimeout:  defaultConnectTimeout,
		requestTimeout:  defaultRequestTimeout,
		maxMessageBytes: DefaultMaxMessageBytes,
	}
}

// WithConnectTimeout changes the positive timeout for each CoreBluetooth
// setup or reconnect attempt. The default is 15 seconds.
func WithConnectTimeout(timeout time.Duration) Option {
	return func(config *config) error {
		if timeout <= 0 {
			return errors.New("BLE connect timeout must be greater than zero")
		}
		config.connectTimeout = timeout
		return nil
	}
}

// WithRequestTimeout changes the positive timeout applied by Device to API
// requests. The default is 10 seconds.
func WithRequestTimeout(timeout time.Duration) Option {
	return func(config *config) error {
		if timeout <= 0 {
			return errors.New("BLE request timeout must be greater than zero")
		}
		config.requestTimeout = timeout
		return nil
	}
}

// WithMaxMessageBytes changes the positive in-memory BLE message limit. The
// default is DefaultMaxMessageBytes.
func WithMaxMessageBytes(maximum int64) Option {
	return func(config *config) error {
		if maximum <= 0 {
			return errors.New("BLE maximum message size must be greater than zero")
		}
		config.maxMessageBytes = maximum
		return nil
	}
}
