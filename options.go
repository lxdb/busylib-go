package busylib

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

const DefaultLocalBaseURL = "http://10.0.4.20"

const (
	headerAPIToken  = "X-API-Token"
	headerRequestID = "X-Request-ID"
	headerSessionID = "x-session-id"
	headerAPISemVer = "X-API-Sem-Ver"
)

const defaultTimeout = 10 * time.Second

type EndpointMode string

const (
	EndpointLocal  EndpointMode = "local"
	EndpointRemote EndpointMode = "remote"
)

type VersionNegotiation string

const (
	VersionNegotiationEnabled  VersionNegotiation = "enabled"
	VersionNegotiationDisabled VersionNegotiation = "disabled"
)

type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

type Option func(*clientConfig) error

type clientConfig struct {
	baseURL              string
	baseURLConfigured    bool
	httpClient           *http.Client
	httpClientConfigured bool
	timeout              time.Duration
	endpointMode         EndpointMode
	localAccessKey       string
	sessionID            string
	requestIDGenerator   func() string
	retryPolicy          RetryPolicy
	versionNegotiation   VersionNegotiation
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		baseURL:            DefaultLocalBaseURL,
		httpClient:         http.DefaultClient,
		timeout:            defaultTimeout,
		endpointMode:       EndpointLocal,
		requestIDGenerator: randomRequestID,
		retryPolicy: RetryPolicy{
			MaxAttempts: 1,
		},
		versionNegotiation: VersionNegotiationEnabled,
	}
}

func WithBaseURL(baseURL string) Option {
	return func(config *clientConfig) error {
		if baseURL == "" {
			return errors.New("base URL must not be empty")
		}
		config.baseURL = baseURL
		config.baseURLConfigured = true
		return nil
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(config *clientConfig) error {
		if httpClient == nil {
			return errors.New("HTTP client must not be nil")
		}
		config.httpClient = httpClient
		config.httpClientConfigured = true
		return nil
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(config *clientConfig) error {
		if timeout < 0 {
			return errors.New("timeout must not be negative")
		}
		config.timeout = timeout
		return nil
	}
}

func WithEndpointMode(mode EndpointMode) Option {
	return func(config *clientConfig) error {
		switch mode {
		case EndpointLocal, EndpointRemote:
			config.endpointMode = mode
			return nil
		default:
			return errors.New("endpoint mode must be local or remote")
		}
	}
}

func WithLocalAccessKey(key string) Option {
	return func(config *clientConfig) error {
		config.localAccessKey = key
		return nil
	}
}

func WithSessionID(sessionID string) Option {
	return func(config *clientConfig) error {
		config.sessionID = sessionID
		return nil
	}
}

func WithRequestIDGenerator(generator func() string) Option {
	return func(config *clientConfig) error {
		if generator == nil {
			return errors.New("request ID generator must not be nil")
		}
		config.requestIDGenerator = generator
		return nil
	}
}

func WithRetryPolicy(policy RetryPolicy) Option {
	return func(config *clientConfig) error {
		if policy.MaxAttempts <= 0 {
			return errors.New("retry policy MaxAttempts must be greater than zero")
		}
		if policy.Backoff < 0 {
			return errors.New("retry policy Backoff must not be negative")
		}
		config.retryPolicy = policy
		return nil
	}
}

func WithVersionNegotiation(mode VersionNegotiation) Option {
	return func(config *clientConfig) error {
		switch mode {
		case VersionNegotiationEnabled, VersionNegotiationDisabled:
			config.versionNegotiation = mode
			return nil
		default:
			return errors.New("version negotiation must be enabled or disabled")
		}
	}
}

func randomRequestID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(data[:])
}
