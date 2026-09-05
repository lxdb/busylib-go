package ble

import (
	"cmp"
	"context"
	"encoding/hex"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	busylib "github.com/lxdb/busylib-go"
	"github.com/lxdb/busylib-go/stream"
)

// The synthetic origin lets the root client construct normal HTTP requests.
// httpTransport intercepts them before DNS or TCP resolution.
const syntheticBaseURL = "http://busybar.ble.invalid"

// Identifier is an opaque CoreBluetooth UUID assigned by macOS. It is not a
// Bluetooth MAC address.
type Identifier string

// Peripheral describes one BUSY Bar observed during a bounded scan.
type Peripheral struct {
	// Identifier selects this peripheral in Connect.
	Identifier Identifier
	// Name is the advertised local name or CoreBluetooth peripheral name. It can
	// be empty.
	Name string
	// RSSI is the received signal strength in dBm.
	RSSI int
}

// Scan discovers connectable BUSY Bars advertising service 0x308A for the
// supplied positive duration. Results are unique by identifier and ordered by
// strongest RSSI. It returns ErrNotFound when the completed scan is empty.
func Scan(ctx context.Context, duration time.Duration) ([]Peripheral, error) {
	return scan(ctx, duration, newPlatformBackend())
}

func scan(ctx context.Context, duration time.Duration, backend backend) ([]Peripheral, error) {
	if ctx == nil {
		return nil, errors.New("BLE scan context must not be nil")
	}
	if duration <= 0 {
		return nil, errors.New("BLE scan duration must be greater than zero")
	}
	peripherals, err := backend.Scan(ctx, duration)
	if err != nil {
		return nil, err
	}
	byIdentifier := make(map[Identifier]Peripheral, len(peripherals))
	for _, peripheral := range peripherals {
		if strings.TrimSpace(string(peripheral.Identifier)) == "" {
			continue
		}
		current, exists := byIdentifier[peripheral.Identifier]
		if !exists || peripheral.RSSI > current.RSSI {
			byIdentifier[peripheral.Identifier] = peripheral
		}
	}
	if len(byIdentifier) == 0 {
		return nil, ErrNotFound
	}
	result := slices.Collect(maps.Values(byIdentifier))
	slices.SortFunc(result, func(left, right Peripheral) int {
		if left.RSSI != right.RSSI {
			return cmp.Compare(right.RSSI, left.RSSI)
		}
		return cmp.Compare(left.Identifier, right.Identifier)
	})
	return result, nil
}

// Client owns one CoreBluetooth connection, its NUS HTTP transport, and any
// status stream created from it.
type Client struct {
	connection connection
	transport  *httpTransport
	device     *busylib.Client
	config     config

	mu        sync.Mutex
	closed    bool
	active    *statusStream
	closeOnce sync.Once
	closeErr  error
}

// Connect retrieves a known CoreBluetooth peripheral by the identifier
// returned by Scan, connects it, validates the BUSY Bar GATT map, and enables
// NUS response notifications. It never scans or changes pairing state.
func Connect(ctx context.Context, identifier Identifier, options ...Option) (*Client, error) {
	config := defaultConfig()
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&config); err != nil {
			return nil, err
		}
	}
	return connect(ctx, identifier, config, newPlatformBackend())
}

func connect(ctx context.Context, identifier Identifier, config config, backend backend) (*Client, error) {
	if ctx == nil {
		return nil, errors.New("BLE connect context must not be nil")
	}
	identifier = Identifier(strings.TrimSpace(string(identifier)))
	if identifier == "" {
		return nil, errors.New("BLE peripheral identifier must not be empty")
	}
	if err := validateIdentifier(identifier); err != nil {
		return nil, err
	}
	connection, err := backend.Connect(ctx, identifier, config.connectTimeout)
	if err != nil {
		return nil, err
	}
	fragmentLimit := min(connection.MaximumWriteValueLength(), firmwareWriteLimit)
	if fragmentLimit <= 0 {
		_ = connection.Close()
		return nil, &Error{Operation: "read write limit", Err: ErrProtocol}
	}
	transport, err := newHTTPTransport(connection, fragmentLimit, config.maxMessageBytes)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	device, err := busylib.NewClient(
		busylib.WithEndpointMode(busylib.EndpointLocal),
		busylib.WithBaseURL(syntheticBaseURL),
		busylib.WithHTTPClient(&http.Client{Transport: transport}),
		busylib.WithTimeout(config.requestTimeout),
		busylib.WithRetryPolicy(busylib.RetryPolicy{MaxAttempts: 1}),
		busylib.WithVersionNegotiation(busylib.VersionNegotiationDisabled),
		busylib.WithMaxResponseBytes(config.maxMessageBytes),
	)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	client := &Client{connection: connection, transport: transport, device: device, config: config}
	connection.SetHTTPNotificationHandler(transport.handleNotification)
	connection.SetHTTPErrorHandler(transport.handleReceiveError)
	connection.SetStateErrorHandler(client.handleStateReceiveError)
	connection.SetDisconnectHandler(client.handleDisconnect)
	return client, nil
}

func validateIdentifier(identifier Identifier) error {
	value := string(identifier)
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return errors.New("BLE peripheral identifier must be a CoreBluetooth UUID")
	}
	hexadecimal := strings.ReplaceAll(value, "-", "")
	if _, err := hex.DecodeString(hexadecimal); err != nil {
		return errors.New("BLE peripheral identifier must be a CoreBluetooth UUID")
	}
	return nil
}

// Device returns the existing root BUSY Bar API client configured with the BLE
// NUS transport. Calls fail after Close or while the physical connection is
// unavailable.
func (c *Client) Device() *busylib.Client { return c.device }

// NewStatusStream creates the client's sole active FFE1 status stream. It uses
// the shared stream contract and does not send status messages through the NUS
// HTTP transport. The caller owns its one-shot lifecycle and must observe
// completion through Stop or Wait.
func (c *Client) NewStatusStream(options ...stream.Option) (stream.Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, ErrClosed
	}
	if c.active != nil {
		return nil, errors.New("BLE client already has an active status stream")
	}
	stream, err := newStatusStream(c, options, func(stream *statusStream) {
		c.mu.Lock()
		if c.active == stream {
			c.active = nil
		}
		c.mu.Unlock()
	})
	if err != nil {
		return nil, err
	}
	c.active = stream
	return stream, nil
}

func (c *Client) handleDisconnect(err error) {
	if err == nil {
		err = ErrDisconnected
	}
	c.transport.handleDisconnect(err)
	c.mu.Lock()
	active := c.active
	c.mu.Unlock()
	if active != nil {
		active.notifyDisconnect(err)
	}
}

func (c *Client) handleStateReceiveError(err error) {
	c.mu.Lock()
	active := c.active
	c.mu.Unlock()
	if active != nil {
		active.signalFatal(err)
	}
}

// Close stops the active status stream, disables notifications, and closes the
// CoreBluetooth connection. Concurrent and repeated calls return one result.
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
	return errors.Join(streamErr, c.transport.Close(), c.connection.Close())
}
