package busylib

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

// DefaultLocalBaseURL is the BUSY Bar USB network endpoint.
const DefaultLocalBaseURL = "http://10.0.4.20"

const (
	headerAPIToken  = "X-API-Token"
	headerRequestID = "X-Request-ID"
	headerSessionID = "x-session-id"
	headerAPISemVer = "X-API-Sem-Ver"
)

const defaultTimeout = 10 * time.Second

// DefaultMaxResponseBytes bounds responses buffered in memory.
const DefaultMaxResponseBytes int64 = 1 << 20

// EndpointMode selects local-device or remote-service request rules.
type EndpointMode string

const (
	// EndpointLocal sends requests directly to a BUSY Bar.
	EndpointLocal EndpointMode = "local"
	// EndpointRemote sends requests through an explicit remote transport.
	EndpointRemote EndpointMode = "remote"
)

// VersionNegotiation controls automatic device API version discovery.
type VersionNegotiation string

const (
	// VersionNegotiationEnabled adds the discovered API version to requests.
	VersionNegotiationEnabled VersionNegotiation = "enabled"
	// VersionNegotiationDisabled sends requests without API version discovery.
	VersionNegotiationDisabled VersionNegotiation = "disabled"
)

// RetryPolicy controls transport retries for repeatable GET, HEAD, and OPTIONS
// requests. MaxAttempts includes the initial attempt. A value of one disables
// retries. Mutating requests are never retried automatically.
type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

// Option configures a Client during NewClient.
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
	maxResponseBytes     int64
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
		maxResponseBytes:   DefaultMaxResponseBytes,
	}
}

// WithMaxResponseBytes changes the maximum response size buffered in memory.
// Use Storage.ReadTo for larger storage files.
func WithMaxResponseBytes(maximum int64) Option {
	return func(config *clientConfig) error {
		if maximum <= 0 {
			return errors.New("maximum response size must be greater than zero")
		}
		config.maxResponseBytes = maximum
		return nil
	}
}

// WithBaseURL sets the device or remote-service HTTP endpoint.
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

// WithHTTPClient sets the HTTP client used for all API requests.
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

// WithTimeout limits each request when its context has no earlier deadline.
func WithTimeout(timeout time.Duration) Option {
	return func(config *clientConfig) error {
		if timeout < 0 {
			return errors.New("timeout must not be negative")
		}
		config.timeout = timeout
		return nil
	}
}

// WithEndpointMode selects local-device or remote-service request behavior.
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

// WithLocalAccessKey adds a local device API token to requests.
// Remote mode does not accept this option.
func WithLocalAccessKey(key string) Option {
	return func(config *clientConfig) error {
		config.localAccessKey = key
		return nil
	}
}

// WithSessionID adds a stable session identifier to requests.
func WithSessionID(sessionID string) Option {
	return func(config *clientConfig) error {
		config.sessionID = sessionID
		return nil
	}
}

// WithRequestIDGenerator sets the function used when a request has no ID.
// The function can be called concurrently.
func WithRequestIDGenerator(generator func() string) Option {
	return func(config *clientConfig) error {
		if generator == nil {
			return errors.New("request ID generator must not be nil")
		}
		config.requestIDGenerator = generator
		return nil
	}
}

// WithRetryPolicy sets the retry count and backoff for safe, repeatable
// requests.
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

// WithVersionNegotiation controls automatic API version discovery and headers.
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
