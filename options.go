package busylib

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

// DefaultLocalBaseURL is the BUSY Bar USB-network HTTP endpoint.
const DefaultLocalBaseURL = "http://10.0.4.20"

const (
	headerAPIToken  = "X-API-Token"
	headerRequestID = "X-Request-ID"
	headerSessionID = "x-session-id"
	headerAPISemVer = "X-API-Sem-Ver"
)

const defaultTimeout = 10 * time.Second

// DefaultMaxResponseBytes is the maximum response body buffered in memory by
// default.
const DefaultMaxResponseBytes int64 = 1 << 20

// EndpointMode selects local-device or remote-transport request rules.
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

// Option configures a Client during NewClient. A nil Option is ignored.
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

// WithMaxResponseBytes changes the positive response-size limit used by all
// buffered response modes. Use StorageService.ReadTo for larger storage files.
func WithMaxResponseBytes(maximum int64) Option {
	return func(config *clientConfig) error {
		if maximum <= 0 {
			return errors.New("maximum response size must be greater than zero")
		}
		config.maxResponseBytes = maximum
		return nil
	}
}

// WithBaseURL sets the endpoint origin. A hostname or IP address without a
// scheme uses HTTP. Any path, query, or fragment is discarded.
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

// WithHTTPClient sets the HTTP client used for all API requests. Client does
// not close the supplied client or its underlying transport.
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

// WithTimeout limits each request when its context has no earlier deadline. A
// zero duration disables the client timeout; a negative duration is invalid.
func WithTimeout(timeout time.Duration) Option {
	return func(config *clientConfig) error {
		if timeout < 0 {
			return errors.New("timeout must not be negative")
		}
		config.timeout = timeout
		return nil
	}
}

// WithEndpointMode selects local-device or remote-transport request behavior.
// Remote mode also requires WithBaseURL and WithHTTPClient and rejects
// WithLocalAccessKey. Applications normally use remote.NewClient instead.
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

// WithLocalAccessKey adds a local device API token to requests. Remote mode
// rejects this option. Callers must not also set X-API-Token in Request.Header.
func WithLocalAccessKey(key string) Option {
	return func(config *clientConfig) error {
		config.localAccessKey = key
		return nil
	}
}

// WithSessionID adds a default x-session-id header to requests. Request.SessionID
// overrides it for one request.
func WithSessionID(sessionID string) Option {
	return func(config *clientConfig) error {
		config.sessionID = sessionID
		return nil
	}
}

// WithRequestIDGenerator sets the function used when neither Request.RequestID
// nor X-Request-ID is present. The function can be called concurrently.
func WithRequestIDGenerator(generator func() string) Option {
	return func(config *clientConfig) error {
		if generator == nil {
			return errors.New("request ID generator must not be nil")
		}
		config.requestIDGenerator = generator
		return nil
	}
}

// WithRetryPolicy sets the retry count and fixed backoff for repeatable GET,
// HEAD, and OPTIONS requests. MaxAttempts includes the initial attempt.
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

// WithVersionNegotiation controls automatic API-version discovery and the
// X-API-Sem-Ver request header. Enabled negotiation caches successful discovery.
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
