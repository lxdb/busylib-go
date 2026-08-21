package busylib

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client sends local or remote HTTP requests to one BUSY Bar API endpoint.
// A Client is safe for concurrent use.
type Client struct {
	baseURL            *url.URL
	httpClient         *http.Client
	timeout            time.Duration
	endpointMode       EndpointMode
	localAccessKey     string
	sessionID          string
	requestIDGenerator func() string
	retryPolicy        RetryPolicy
	versionNegotiation VersionNegotiation
	maxResponseBytes   int64

	versionMu       sync.Mutex
	apiSemVer       string
	versionInFlight *versionRefresh
}

// NewClient creates a client with the supplied options.
// It returns an error when an option or endpoint configuration is invalid.
func NewClient(options ...Option) (*Client, error) {
	config := defaultClientConfig()
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&config); err != nil {
			return nil, err
		}
	}

	baseURL, err := normalizeBaseURL(config.baseURL)
	if err != nil {
		return nil, err
	}

	if config.endpointMode == EndpointRemote {
		if !config.baseURLConfigured {
			return nil, errors.New("remote mode requires an explicit base URL")
		}
		if !config.httpClientConfigured {
			return nil, errors.New("remote mode requires an explicit HTTP client")
		}
		if config.localAccessKey != "" {
			return nil, errors.New("remote mode does not support a local access key")
		}
	}

	return &Client{
		baseURL:            baseURL,
		httpClient:         config.httpClient,
		timeout:            config.timeout,
		endpointMode:       config.endpointMode,
		localAccessKey:     config.localAccessKey,
		sessionID:          config.sessionID,
		requestIDGenerator: config.requestIDGenerator,
		retryPolicy:        config.retryPolicy,
		versionNegotiation: config.versionNegotiation,
		maxResponseBytes:   config.maxResponseBytes,
	}, nil
}

// BaseURL returns the normalized HTTP endpoint used by the client.
func (c *Client) BaseURL() string {
	return c.baseURL.String()
}

func normalizeBaseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("base URL must not be empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("base URL scheme must be http or https")
	}
	if parsed.Host == "" {
		return nil, errors.New("base URL host must not be empty")
	}

	return &url.URL{
		Scheme: parsed.Scheme,
		Host:   parsed.Host,
	}, nil
}
