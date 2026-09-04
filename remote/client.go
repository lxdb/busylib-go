package remote

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync"

	busylib "github.com/lxdb/busylib-go"
	"github.com/lxdb/busylib-go/stream"
)

const syntheticBaseURL = "http://busybar.remote.invalid"

// Client exposes the root BUSY Bar client over the firmware's MQTT HTTP
// protocol. It owns its HTTP-response subscription and any status stream that
// it creates, but it does not own the supplied Transport.
type Client struct {
	transport Transport
	config    clientConfig
	http      *httpRoundTripper
	device    *busylib.Client

	mu        sync.Mutex
	closed    bool
	active    *statusStream
	closeOnce sync.Once
	closeErr  error
}

// NewClient creates a remote client over a caller-owned MQTT 5 transport.
// sessionID must be one safe MQTT topic segment. Unless WithClientID supplies a
// stable value, NewClient generates a random client ID for response topics.
func NewClient(transport Transport, sessionID string, options ...Option) (*Client, error) {
	if transport == nil {
		return nil, errors.New("remote transport must not be nil")
	}
	if err := validateTopicSegment("session ID", sessionID); err != nil {
		return nil, err
	}
	config := defaultConfig()
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&config); err != nil {
			return nil, err
		}
	}
	if config.clientID == "" {
		clientID, err := randomHex(16)
		if err != nil {
			return nil, err
		}
		config.clientID = clientID
	}
	config.sessionID = sessionID

	httpTransport := newHTTPRoundTripper(transport, sessionID, config.clientID, config.maxMessageBytes)
	rootOptions := []busylib.Option{
		busylib.WithEndpointMode(busylib.EndpointRemote),
		busylib.WithBaseURL(syntheticBaseURL),
		busylib.WithHTTPClient(&http.Client{Transport: httpTransport}),
		busylib.WithTimeout(config.requestTimeout),
		busylib.WithMaxResponseBytes(config.maxMessageBytes),
	}
	if config.requestSessionID != "" {
		rootOptions = append(rootOptions, busylib.WithSessionID(config.requestSessionID))
	}
	device, err := busylib.NewClient(rootOptions...)
	if err != nil {
		return nil, err
	}
	return &Client{
		transport: transport,
		config:    config,
		http:      httpTransport,
		device:    device,
	}, nil
}

// Device returns the BUSY Bar HTTP API client backed by remote MQTT. Calls made
// through the returned client fail after Close.
func (c *Client) Device() *busylib.Client { return c.device }

// NewStatusStream creates the client's sole active remote MQTT status stream.
// The caller must start and eventually stop or wait for the stream. Another
// stream can be created after the current stream finishes.
func (c *Client) NewStatusStream(options ...stream.Option) (stream.Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, ErrClosed
	}
	if c.active != nil {
		return nil, errors.New("remote client already has an active status stream")
	}
	statusStream, err := newStatusStream(c.transport, c.config, options, func(stream *statusStream) {
		c.mu.Lock()
		if c.active == stream {
			c.active = nil
		}
		c.mu.Unlock()
	})
	if err != nil {
		return nil, err
	}
	c.active = statusStream
	return statusStream, nil
}

// Close stops the active stream and all client-owned subscriptions. It never
// closes the caller's Transport. Concurrent and repeated calls return the same
// completion result.
func (c *Client) Close() error {
	c.closeOnce.Do(func() { c.closeErr = c.close() })
	return c.closeErr
}

func (c *Client) close() error {
	c.mu.Lock()
	c.closed = true
	active := c.active
	c.mu.Unlock()
	var streamErr error
	if active != nil {
		streamErr = active.Stop()
	}
	return errors.Join(streamErr, c.http.Close())
}

func randomHex(bytesCount int) (string, error) {
	data := make([]byte, bytesCount)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate remote identifier: %w", err)
	}
	return hex.EncodeToString(data), nil
}
