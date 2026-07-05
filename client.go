package busylib

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Client struct {
	baseURL            *url.URL
	httpClient         *http.Client
	timeout            time.Duration
	endpointMode       EndpointMode
	localAccessKey     string
	cloudBearerToken   string
	sessionID          string
	requestIDGenerator func() string
	retryPolicy        RetryPolicy
	versionNegotiation VersionNegotiation

	versionMu       sync.Mutex
	apiSemVer       string
	versionInFlight *versionRefresh
}

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

	if config.endpointMode == EndpointProxy {
		if !config.baseURLConfigured {
			return nil, errors.New("proxy mode requires an explicit base URL")
		}
		if baseURL.Scheme != "https" {
			return nil, validationError("", "", "proxy mode requires an https base URL", nil)
		}
		if config.cloudBearerToken == "" {
			return nil, errors.New("proxy mode requires a cloud bearer token")
		}
	}

	return &Client{
		baseURL:            baseURL,
		httpClient:         config.httpClient,
		timeout:            config.timeout,
		endpointMode:       config.endpointMode,
		localAccessKey:     config.localAccessKey,
		cloudBearerToken:   config.cloudBearerToken,
		sessionID:          config.sessionID,
		requestIDGenerator: config.requestIDGenerator,
		retryPolicy:        config.retryPolicy,
		versionNegotiation: config.versionNegotiation,
	}, nil
}

func (c *Client) BaseURL() string {
	return c.baseURL.String()
}

func (c *Client) setCachedAPISemVerForTest(version string) {
	c.versionMu.Lock()
	defer c.versionMu.Unlock()
	c.apiSemVer = version
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
