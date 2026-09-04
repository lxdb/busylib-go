package remote

import (
	"errors"
	"strings"
	"time"
)

const (
	defaultRequestTimeout = 10 * time.Second
	defaultStreamLease    = 60 * time.Second
	// DefaultMaxMessageBytes is the 1 MiB limit applied to MQTT payloads by
	// default.
	DefaultMaxMessageBytes int64 = 1 << 20
)

// MessageLimit configures the firmware stream publisher's packet rate. A zero
// value leaves the publisher unlimited; otherwise both fields must be positive.
type MessageLimit struct {
	MaxCount uint32
	Interval time.Duration
}

// Option configures a remote Client. NewClient ignores nil options.
type Option func(*clientConfig) error

type clientConfig struct {
	clientID         string
	sessionID        string
	requestTimeout   time.Duration
	requestSessionID string
	streamLease      time.Duration
	streamLimit      MessageLimit
	maxMessageBytes  int64
}

func defaultConfig() clientConfig {
	return clientConfig{
		requestTimeout:  defaultRequestTimeout,
		streamLease:     defaultStreamLease,
		maxMessageBytes: DefaultMaxMessageBytes,
	}
}

// WithMaxMessageBytes changes the positive MQTT payload limit for HTTP
// responses and status-stream messages.
func WithMaxMessageBytes(maximum int64) Option {
	return func(config *clientConfig) error {
		if maximum <= 0 {
			return errors.New("remote maximum message size must be greater than zero")
		}
		config.maxMessageBytes = maximum
		return nil
	}
}

// WithClientID sets the safe MQTT topic segment that distinguishes this client.
// NewClient generates a random ID when this option is absent.
func WithClientID(clientID string) Option {
	return func(config *clientConfig) error {
		if err := validateTopicSegment("client ID", clientID); err != nil {
			return err
		}
		config.clientID = clientID
		return nil
	}
}

// WithRequestTimeout sets the positive timeout for device API requests and
// remote stream setup operations. The default is 10 seconds.
func WithRequestTimeout(timeout time.Duration) Option {
	return func(config *clientConfig) error {
		if timeout <= 0 {
			return errors.New("remote request timeout must be greater than zero")
		}
		config.requestTimeout = timeout
		return nil
	}
}

// WithRequestSessionID sets the optional x-session-id header inside tunneled
// HTTP requests. It is distinct from the firmware session ID in MQTT topics.
func WithRequestSessionID(sessionID string) Option {
	return func(config *clientConfig) error {
		if strings.ContainsAny(sessionID, "\r\n") {
			return errors.New("remote request session ID must not contain a newline")
		}
		config.requestSessionID = sessionID
		return nil
	}
}

// WithStreamLease sets the firmware stream lease. The client renews the lease
// halfway through its duration. The value must be a positive whole number of
// seconds; the default is 60 seconds.
func WithStreamLease(lease time.Duration) Option {
	return func(config *clientConfig) error {
		if lease <= 0 || lease%time.Second != 0 {
			return errors.New("remote stream lease must be a positive whole number of seconds")
		}
		config.streamLease = lease
		return nil
	}
}

// WithStreamMessageLimit configures an optional firmware stream rate limit.
// Set both fields to zero to disable the limit.
func WithStreamMessageLimit(limit MessageLimit) Option {
	return func(config *clientConfig) error {
		if limit.MaxCount == 0 && limit.Interval == 0 {
			config.streamLimit = limit
			return nil
		}
		if limit.MaxCount == 0 || limit.Interval <= 0 {
			return errors.New("remote stream message limit requires a positive count and interval")
		}
		config.streamLimit = limit
		return nil
	}
}

func validateTopicSegment(name, value string) error {
	if value == "" {
		return errors.New("remote " + name + " must not be empty")
	}
	if len(value) > 64 {
		return errors.New("remote " + name + " must not exceed 64 bytes")
	}
	if strings.ContainsAny(value, "/+#\x00") {
		return errors.New("remote " + name + " must be one safe MQTT topic segment")
	}
	return nil
}
