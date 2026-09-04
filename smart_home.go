package busylib

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// SmartHomeService controls smart-home pairing and switch state.
type SmartHomeService struct {
	client *Client
}

// SmartHome returns the smart-home API.
func (c *Client) SmartHome() SmartHomeService { return SmartHomeService{client: c} }

// SmartHomePairingInfo reports Matter fabric and pairing state.
type SmartHomePairingInfo struct {
	FabricCount         int                       `json:"fabric_count"`
	LatestPairingStatus SmartHomePairingStatusRef `json:"latest_pairing_status"`
}

// SmartHomePairingStatusRef records a pairing state and its timestamp.
type SmartHomePairingStatusRef struct {
	Value     SmartHomePairingStatus `json:"value"`
	Timestamp int64                  `json:"timestamp,omitempty"`
}

// SmartHomePairingStatus identifies the latest Matter pairing result.
type SmartHomePairingStatus string

const (
	// SmartHomePairingNeverStarted means pairing has not started.
	SmartHomePairingNeverStarted SmartHomePairingStatus = "never_started"
	// SmartHomePairingStarted means pairing is in progress.
	SmartHomePairingStarted SmartHomePairingStatus = "started"
	// SmartHomePairingCompletedSuccessfully means pairing succeeded.
	SmartHomePairingCompletedSuccessfully SmartHomePairingStatus = "completed_successfully"
	// SmartHomePairingFailed means pairing failed.
	SmartHomePairingFailed SmartHomePairingStatus = "failed"
)

// SmartHomePairingPayload contains codes for a temporary Matter pairing
// session. Treat QRCode and ManualCode as temporary credentials.
type SmartHomePairingPayload struct {
	AvailableUntil string `json:"available_until"`
	QRCode         string `json:"qr_code"`
	ManualCode     string `json:"manual_code"`
}

// SmartHomeSwitchState reports the current Matter switch state.
type SmartHomeSwitchState struct {
	State bool `json:"state"`
}

// SmartHomeSwitchUpdate changes either the current Matter switch state, its
// startup behavior, or both. Pointer state preserves the firmware's partial
// update contract.
type SmartHomeSwitchUpdate struct {
	State   *bool                  `json:"state,omitempty"`
	Startup SmartHomeSwitchStartup `json:"startup,omitempty"`
}

// SmartHomeSwitchStartup controls the switch state after startup.
type SmartHomeSwitchStartup string

const (
	// SmartHomeSwitchStartupOff starts with the switch off.
	SmartHomeSwitchStartupOff SmartHomeSwitchStartup = "off"
	// SmartHomeSwitchStartupOn starts with the switch on.
	SmartHomeSwitchStartupOn SmartHomeSwitchStartup = "on"
	// SmartHomeSwitchStartupToggle inverts the previous switch state.
	SmartHomeSwitchStartupToggle SmartHomeSwitchStartup = "toggle"
	// SmartHomeSwitchStartupLast restores the previous switch state.
	SmartHomeSwitchStartupLast SmartHomeSwitchStartup = "last"
)

// PairingStatus returns the current smart-home pairing state.
func (s SmartHomeService) PairingStatus(ctx context.Context) (SmartHomePairingInfo, error) {
	var out SmartHomePairingInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/smart_home/pairing", nil, nil, &out)
	return out, err
}

// StartPairing starts smart-home pairing and returns setup data.
func (s SmartHomeService) StartPairing(ctx context.Context) (SmartHomePairingPayload, error) {
	var out SmartHomePairingPayload
	err := s.client.doJSON(ctx, http.MethodPost, "/api/smart_home/pairing", nil, nil, &out)
	return out, err
}

// ForgetPairings permanently removes all saved smart-home pairings.
func (s SmartHomeService) ForgetPairings(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/smart_home/pairing", nil, nil)
}

// SwitchState returns the current smart-home switch configuration.
func (s SmartHomeService) SwitchState(ctx context.Context) (SmartHomeSwitchState, error) {
	var out SmartHomeSwitchState
	err := s.client.doJSON(ctx, http.MethodGet, "/api/smart_home/switch", nil, nil, &out)
	return out, err
}

// SetSwitchState validates and changes the supplied smart-home switch fields.
// Nil pointer fields preserve their current values.
func (s SmartHomeService) SetSwitchState(ctx context.Context, update SmartHomeSwitchUpdate) error {
	if err := update.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/smart_home/switch", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/smart_home/switch", nil, JSONBody(update))
}

// Validate reports whether a smart-home switch update has supported values.
func (update SmartHomeSwitchUpdate) Validate() error {
	if update.State == nil && update.Startup == "" {
		return errors.New("state or startup must be provided")
	}
	if update.Startup != "" && !validSmartHomeSwitchStartup(update.Startup) {
		return fmt.Errorf("startup %q is not supported", update.Startup)
	}
	return nil
}

func validSmartHomeSwitchStartup(value SmartHomeSwitchStartup) bool {
	switch value {
	case SmartHomeSwitchStartupOff, SmartHomeSwitchStartupOn, SmartHomeSwitchStartupToggle, SmartHomeSwitchStartupLast:
		return true
	default:
		return false
	}
}
