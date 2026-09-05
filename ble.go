package busylib

import (
	"context"
	"net/http"
)

// BLEService reads and controls Bluetooth Low Energy state and pairing.
type BLEService struct {
	client *Client
}

// BLE returns the Bluetooth Low Energy API.
func (c *Client) BLE() BLEService { return BLEService{client: c} }

// BLEStatus reports the current Bluetooth Low Energy state and address.
type BLEStatus struct {
	State   BLEState `json:"status"`
	Address string   `json:"address,omitempty"`
}

// BLEState identifies a firmware Bluetooth Low Energy lifecycle state.
type BLEState string

const (
	// BLEStateReset means the Bluetooth subsystem is reset.
	BLEStateReset BLEState = "reset"
	// BLEStateInitialization means the Bluetooth subsystem is starting.
	BLEStateInitialization BLEState = "initialization"
	// BLEStateDisabled means Bluetooth is disabled.
	BLEStateDisabled BLEState = "disabled"
	// BLEStateEnabled means Bluetooth is enabled but not connectable.
	BLEStateEnabled BLEState = "enabled"
	// BLEStateConnectable means another device can connect.
	BLEStateConnectable BLEState = "connectable"
	// BLEStateConnected means another device is connected.
	BLEStateConnected BLEState = "connected"
	// BLEStateInternalError means the Bluetooth subsystem failed.
	BLEStateInternalError BLEState = "internal error"
)

// Status returns Bluetooth Low Energy state and pairing details.
func (s BLEService) Status(ctx context.Context) (BLEStatus, error) {
	var out BLEStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/ble/status", nil, nil, &out)
	return out, err
}

// Enable turns on Bluetooth Low Energy support.
func (s BLEService) Enable(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodPost, "/api/ble/enable", nil, nil)
}

// Disable turns off Bluetooth Low Energy support.
func (s BLEService) Disable(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodPost, "/api/ble/disable", nil, nil)
}

// RemovePairing removes the saved Bluetooth Low Energy pairing.
func (s BLEService) RemovePairing(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/ble/pairing", nil, nil)
}
