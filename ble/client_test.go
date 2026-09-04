package ble

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	busylib "github.com/lxdb/busylib-go"
)

type fakeBackend struct {
	peripherals []Peripheral
	scanErr     error
	connection  *fakeConnection
	connectErr  error
	connectedID Identifier
}

func (b *fakeBackend) Scan(context.Context, time.Duration) ([]Peripheral, error) {
	return slices.Clone(b.peripherals), b.scanErr
}

func (b *fakeBackend) Connect(_ context.Context, identifier Identifier, _ time.Duration) (connection, error) {
	b.connectedID = identifier
	if b.connectErr != nil {
		return nil, b.connectErr
	}
	return b.connection, nil
}

type fakeConnection struct {
	mu sync.Mutex

	httpHandler       func([]byte)
	httpErrorHandler  func(error)
	disconnectHandler func(error)
	stateHandler      func([]byte)
	stateErrorHandler func(error)
	response          []byte
	respond           func([]byte) []byte
	writes            [][]byte
	reconnectErr      error
	reconnectStarted  chan struct{}
	reconnectRelease  chan struct{}
	reconnects        int
	stateEnables      int
	stateDisables     int
	closes            int
}

func (c *fakeConnection) MaximumWriteValueLength() int { return 512 }

func (c *fakeConnection) SetHTTPNotificationHandler(handler func([]byte)) {
	c.mu.Lock()
	c.httpHandler = handler
	c.mu.Unlock()
}

func (c *fakeConnection) SetHTTPErrorHandler(handler func(error)) {
	c.mu.Lock()
	c.httpErrorHandler = handler
	c.mu.Unlock()
}

func (c *fakeConnection) SetStateErrorHandler(handler func(error)) {
	c.mu.Lock()
	c.stateErrorHandler = handler
	c.mu.Unlock()
}

func (c *fakeConnection) SetDisconnectHandler(handler func(error)) {
	c.mu.Lock()
	c.disconnectHandler = handler
	c.mu.Unlock()
}

func (c *fakeConnection) Write(_ context.Context, fragment []byte) error {
	c.mu.Lock()
	c.writes = append(c.writes, bytes.Clone(fragment))
	handler := c.httpHandler
	response := bytes.Clone(c.response)
	c.response = nil
	if c.respond != nil {
		response = bytes.Clone(c.respond(fragment))
	}
	c.mu.Unlock()
	if handler != nil && len(response) != 0 {
		handler(response)
	}
	return nil
}

func (c *fakeConnection) EnableStateNotifications(_ context.Context, handler func([]byte)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stateEnables++
	c.stateHandler = handler
	return nil
}

func (c *fakeConnection) DisableStateNotifications(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stateDisables++
	c.stateHandler = nil
	return nil
}

func (c *fakeConnection) Reconnect(context.Context, time.Duration) error {
	c.mu.Lock()
	c.reconnects++
	started := c.reconnectStarted
	release := c.reconnectRelease
	err := c.reconnectErr
	c.mu.Unlock()
	if started != nil {
		select {
		case <-started:
		default:
			close(started)
		}
	}
	if release != nil {
		<-release
	}
	return err
}

func (c *fakeConnection) Close() error {
	c.mu.Lock()
	c.closes++
	c.mu.Unlock()
	return nil
}

func (c *fakeConnection) emitDisconnect(err error) {
	c.mu.Lock()
	handler := c.disconnectHandler
	c.mu.Unlock()
	if handler != nil {
		handler(err)
	}
}

func (c *fakeConnection) emitState(packet []byte) {
	c.mu.Lock()
	handler := c.stateHandler
	c.mu.Unlock()
	if handler != nil {
		handler(packet)
	}
}

func TestScanDeduplicatesAndOrdersStrongestPeripheralFirst(t *testing.T) {
	backend := &fakeBackend{peripherals: []Peripheral{
		{Identifier: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", Name: "B", RSSI: -70},
		{Identifier: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Name: "A-old", RSSI: -80},
		{Identifier: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Name: "A", RSSI: -40},
	}}

	got, err := scan(context.Background(), time.Second, backend)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(peripherals) = %d, want 2", len(got))
	}
	if got[0].Identifier != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" || got[0].Name != "A" || got[0].RSSI != -40 {
		t.Fatalf("first peripheral = %+v, want strongest duplicate", got[0])
	}
	if got[1].Identifier != "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" {
		t.Fatalf("second identifier = %q", got[1].Identifier)
	}
}

func TestScanReturnsNotFoundAfterCompletedEmptyScan(t *testing.T) {
	_, err := scan(context.Background(), time.Second, new(fakeBackend))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("scan error = %v, want ErrNotFound", err)
	}
}

func TestConnectRejectsEmptyIdentifierBeforeBackendUse(t *testing.T) {
	backend := &fakeBackend{connectErr: errors.New("backend must not be called")}
	_, err := connect(context.Background(), "", defaultConfig(), backend)
	if err == nil || errors.Is(err, backend.connectErr) {
		t.Fatalf("connect error = %v, want identifier validation", err)
	}
}

func TestConnectRejectsMalformedIdentifierBeforeBackendUse(t *testing.T) {
	backendErr := errors.New("backend must not be called")
	backend := &fakeBackend{connectErr: backendErr}
	_, err := connect(context.Background(), "not-a-corebluetooth-uuid", defaultConfig(), backend)
	if err == nil || errors.Is(err, backendErr) {
		t.Fatalf("connect error = %v, want identifier validation", err)
	}
}

func TestConnectedClientExposesRootDeviceAPI(t *testing.T) {
	connection := &fakeConnection{
		response: []byte("HTTP/1.1 200 OK\r\nContent-Length: 23\r\n\r\n{\"api_semver\":\"27.5.0\"}"),
	}
	backend := &fakeBackend{connection: connection}
	client, err := connect(
		context.Background(),
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		defaultConfig(),
		backend,
	)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { checkCloseClient(t, client) })

	version, err := client.Device().System().Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if version.APISemVer != "27.5.0" {
		t.Fatalf("APISemVer = %q, want 27.5.0", version.APISemVer)
	}
	if backend.connectedID != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
		t.Fatalf("connected identifier = %q", backend.connectedID)
	}
}

func TestConnectedClientUsesDirectDeviceRequestRules(t *testing.T) {
	connection := &fakeConnection{
		response: []byte("HTTP/1.1 200 OK\r\nContent-Length: 67\r\n\r\n{\"count\":1,\"networks\":[{\"ssid\":\"office\",\"security\":\"\",\"rssi\":-42}]}"),
	}
	client := connectedTestClient(t, connection)
	t.Cleanup(func() { checkCloseClient(t, client) })

	networks, err := client.Device().WiFi().Networks(context.Background())
	if err != nil {
		t.Fatalf("Networks: %v", err)
	}
	if networks.Count != 1 || len(networks.Networks) != 1 || networks.Networks[0].SSID != "office" {
		t.Fatalf("Networks = %+v, want one office network", networks)
	}
	connection.mu.Lock()
	defer connection.mu.Unlock()
	request := bytes.Join(connection.writes, nil)
	if !bytes.HasPrefix(request, []byte("GET /api/wifi/networks HTTP/1.1\r\n")) {
		t.Fatalf("BLE request = %q, want Wi-Fi networks request", request)
	}
}

func TestConnectedClientDoesNotReplayMutationForVersionCompatibility(t *testing.T) {
	var inputCalls int
	connection := &fakeConnection{respond: func(request []byte) []byte {
		switch {
		case bytes.HasPrefix(request, []byte("GET /api/version ")):
			return []byte("HTTP/1.1 200 OK\r\nContent-Length: 23\r\n\r\n{\"api_semver\":\"27.5.0\"}")
		case bytes.HasPrefix(request, []byte("POST /api/input?key=ok ")):
			inputCalls++
			if inputCalls == 1 {
				return []byte("HTTP/1.1 405 Method Not Allowed\r\nContent-Length: 28\r\n\r\n{\"error\":\"version mismatch\"}")
			}
			return []byte("HTTP/1.1 200 OK\r\nContent-Length: 15\r\n\r\n{\"status\":\"ok\"}")
		default:
			return []byte("HTTP/1.1 500 Internal Server Error\r\nContent-Length: 0\r\n\r\n")
		}
	}}
	client := connectedTestClient(t, connection)
	t.Cleanup(func() { checkCloseClient(t, client) })

	err := client.Device().Input().SendKey(context.Background(), busylib.InputKeyOK)
	if err == nil {
		t.Fatal("SendKey succeeded after compatibility response, want no automatic replay")
	}
	connection.mu.Lock()
	writes := len(connection.writes)
	connection.mu.Unlock()
	if writes != 1 {
		t.Fatalf("BLE writes = %d, want one mutation without version discovery or replay", writes)
	}
}

func checkCloseClient(t *testing.T, client *Client) {
	t.Helper()
	if err := client.Close(); err != nil {
		t.Errorf("close BLE client: %v", err)
	}
}

func TestClientCloseIsIdempotent(t *testing.T) {
	connection := new(fakeConnection)
	client, err := connect(
		context.Background(),
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		defaultConfig(),
		&fakeBackend{connection: connection},
	)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	connection.mu.Lock()
	defer connection.mu.Unlock()
	if connection.closes != 1 {
		t.Fatalf("connection closes = %d, want 1", connection.closes)
	}
}
