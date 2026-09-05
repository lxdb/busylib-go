package busylib

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
)

var logFilenamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

const maxLogFilenameBytes = 63

// SystemService reads device identity, health, transport, and diagnostic data.
type SystemService struct {
	client *Client
}

// System returns the device system API.
func (c *Client) System() SystemService { return SystemService{client: c} }

// LogDumpResponse identifies a log dump that the device created.
type LogDumpResponse struct {
	Result string `json:"result"`
	Path   string `json:"path"`
}

// VersionInfo contains the semantic version of the firmware HTTP API.
type VersionInfo struct {
	APISemVer string `json:"api_semver"`
}

// NetworkInterfaceInfo identifies the transport used for the current request.
type NetworkInterfaceInfo struct {
	Type TransportType `json:"type"`
}

// TransportType identifies a device network transport.
type TransportType string

const (
	// TransportUSB uses the USB network interface.
	TransportUSB TransportType = "usb"
	// TransportWiFi uses the Wi-Fi network interface.
	TransportWiFi TransportType = "wifi"
)

// Status contains the current device, firmware, system, and power state.
type Status struct {
	Device   DeviceStatus   `json:"device"`
	Firmware FirmwareStatus `json:"firmware"`
	System   SystemStatus   `json:"system"`
	Power    PowerStatus    `json:"power"`
}

// DeviceStatus identifies the device and its provisioned hardware interfaces.
type DeviceStatus struct {
	SerialNumber     string `json:"serial_number"`
	USBMAC           string `json:"usb_mac"`
	WiFiMAC          string `json:"wifi_mac,omitempty"`
	BLEMAC           string `json:"ble_mac,omitempty"`
	OTPValid         bool   `json:"otp_valid"`
	OTPModel         string `json:"otp_model,omitempty"`
	OTPTimestamp     int64  `json:"otp_timestamp,omitempty"`
	FirmwareSecurity string `json:"firmware_security"`
}

// FirmwareStatus identifies the firmware build currently running on the device.
type FirmwareStatus struct {
	Version         string `json:"version"`
	Target          int    `json:"target"`
	Branch          string `json:"branch"`
	BuildDate       string `json:"build_date"`
	CommitHash      string `json:"commit_hash"`
	IntercomVersion string `json:"intercom_version"`
	NWPVersion      string `json:"nwp_version,omitempty"`
	MatterVersion   string `json:"matter_version,omitempty"`
}

// SystemStatus reports the API version, uptime, boot time, and update state.
type SystemStatus struct {
	APISemVer         string `json:"api_semver"`
	Uptime            string `json:"uptime"`
	BootTime          int64  `json:"boot_time"`
	AutoUpdateEnabled bool   `json:"auto_update_enabled"`
}

// PowerStatus reports battery, USB, and charging measurements.
type PowerStatus struct {
	State          PowerState `json:"state"`
	BatteryCharge  int        `json:"battery_charge"`
	BatteryVoltage int        `json:"battery_voltage"`
	BatteryCurrent int        `json:"battery_current"`
	USBVoltage     int        `json:"usb_voltage"`
}

// PowerState identifies whether the battery supplies or receives power.
type PowerState string

const (
	// PowerDischarging means the battery supplies power.
	PowerDischarging PowerState = "discharging"
	// PowerCharging means the battery receives power.
	PowerCharging PowerState = "charging"
	// PowerCharged means charging is complete.
	PowerCharged PowerState = "charged"
)

// Version returns device firmware and API version details.
func (s SystemService) Version(ctx context.Context) (VersionInfo, error) {
	var out VersionInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/version", nil, nil, &out)
	return out, err
}

// Status returns the aggregate device status.
func (s SystemService) Status(ctx context.Context) (Status, error) {
	var out Status
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status", nil, nil, &out)
	return out, err
}

// DeviceStatus returns identity and hardware status.
func (s SystemService) DeviceStatus(ctx context.Context) (DeviceStatus, error) {
	var out DeviceStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status/device", nil, nil, &out)
	return out, err
}

// FirmwareStatus returns firmware version and build status.
func (s SystemService) FirmwareStatus(ctx context.Context) (FirmwareStatus, error) {
	var out FirmwareStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status/firmware", nil, nil, &out)
	return out, err
}

// SystemStatus returns runtime and memory status.
func (s SystemService) SystemStatus(ctx context.Context) (SystemStatus, error) {
	var out SystemStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status/system", nil, nil, &out)
	return out, err
}

// PowerStatus returns power-source and battery status.
func (s SystemService) PowerStatus(ctx context.Context) (PowerStatus, error) {
	var out PowerStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/status/power", nil, nil, &out)
	return out, err
}

// Transport returns the network interface used by the API.
func (s SystemService) Transport(ctx context.Context) (NetworkInterfaceInfo, error) {
	var out NetworkInterfaceInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/transport", nil, nil, &out)
	return out, err
}

// DumpLog creates a device log archive and returns its storage path.
// An empty filename lets the device choose the archive name.
func (s SystemService) DumpLog(ctx context.Context, filename string) (LogDumpResponse, error) {
	var out LogDumpResponse
	if err := validateOptionalLogFilename(filename); err != nil {
		return out, validationError(http.MethodPost, "/api/log_dump", err.Error(), err)
	}
	query := url.Values{}
	if filename != "" {
		query.Set("filename", filename)
	}
	err := s.client.doJSON(ctx, http.MethodPost, "/api/log_dump", query, nil, &out)
	return out, err
}

func validateOptionalLogFilename(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > maxLogFilenameBytes || !logFilenamePattern.MatchString(value) {
		return fmt.Errorf("filename must be 1-%d ASCII letters, digits, underscores, or hyphens", maxLogFilenameBytes)
	}
	return nil
}
