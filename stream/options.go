package stream

import (
	"errors"
	"time"
)

const (
	defaultStaleAfter        = 5 * time.Second
	defaultReconnectDelay    = time.Second
	defaultReconnectAttempts = 5
)

// ReconnectPolicy controls the bounded dial attempts made for an initial
// connection or after an established connection is lost.
type ReconnectPolicy struct {
	MaxAttempts int
	Delay       time.Duration
}

// Options contains the small set of status-stream policies callers can tune.
// Zero values are replaced by the documented defaults.
type Options struct {
	StaleAfter time.Duration
	Reconnect  ReconnectPolicy
}

// Option configures a local status stream.
type Option func(*Options) error

// DefaultOptions returns the status-stream defaults.
func DefaultOptions() Options {
	return Options{
		StaleAfter: defaultStaleAfter,
		Reconnect: ReconnectPolicy{
			MaxAttempts: defaultReconnectAttempts,
			Delay:       defaultReconnectDelay,
		},
	}
}

// WithStaleAfter changes how long the stream waits without a valid protobuf
// state before marking its data stale.
func WithStaleAfter(duration time.Duration) Option {
	return func(options *Options) error {
		if duration <= 0 {
			return errors.New("stream stale duration must be greater than zero")
		}
		options.StaleAfter = duration
		return nil
	}
}

// WithReconnectPolicy changes the bounded connection retry policy.
func WithReconnectPolicy(policy ReconnectPolicy) Option {
	return func(options *Options) error {
		if policy.MaxAttempts <= 0 {
			return errors.New("stream reconnect MaxAttempts must be greater than zero")
		}
		if policy.Delay < 0 {
			return errors.New("stream reconnect Delay must not be negative")
		}
		options.Reconnect = policy
		return nil
	}
}

// ResolveOptions applies options to the defaults.
func ResolveOptions(configurers ...Option) (Options, error) {
	options := DefaultOptions()
	for _, configure := range configurers {
		if configure == nil {
			continue
		}
		if err := configure(&options); err != nil {
			return Options{}, err
		}
	}
	return options, nil
}
