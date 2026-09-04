package busylib

import (
	"encoding/json"

	"github.com/lxdb/busylib-go/display"
)

type successResponse struct {
	Result string `json:"result"`
}

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

// HTTPAccessInfo reports the device HTTP access mode and key state.
type HTTPAccessInfo struct {
	Mode     HTTPAccessMode `json:"mode"`
	KeyValid bool           `json:"key_valid"`
}

// StoredAccessToken identifies an access token without exposing its secret.
type StoredAccessToken struct {
	// ShortID is the first eight token characters and identifies the token for
	// revocation.
	ShortID string `json:"short_id"`
	// DisplayID is a redacted identifier formed from the token prefix and suffix.
	DisplayID string `json:"display_id"`
	// Name is the caller-supplied label for the token.
	Name string `json:"name"`
	// CreatedAt is a Unix millisecond timestamp encoded as a decimal string.
	CreatedAt string `json:"created_at"`
	// LastUsedAt is the latest-use Unix millisecond timestamp encoded as a
	// decimal string. The firmware reports "0" before the token is first used.
	LastUsedAt string `json:"last_used_at"`
}

// MintedAccessToken contains metadata for a newly created token and its
// one-time credential.
type MintedAccessToken struct {
	StoredAccessToken
	// Token is the full credential. The device returns it only when the token is
	// created.
	Token string `json:"token"`
}

// AccessTokensInfo contains the access tokens stored on the device.
type AccessTokensInfo struct {
	Tokens []StoredAccessToken `json:"tokens"`
}

// HTTPAccessMode controls access to the device HTTP API.
type HTTPAccessMode string

const (
	// HTTPAccessDisabled blocks HTTP API access.
	HTTPAccessDisabled HTTPAccessMode = "disabled"
	// HTTPAccessEnabled allows HTTP API access without a key.
	HTTPAccessEnabled HTTPAccessMode = "enabled"
	// HTTPAccessKey requires a valid access key.
	HTTPAccessKey HTTPAccessMode = "key"
)

// NameInfo contains the device name.
type NameInfo struct {
	Name string `json:"name"`
}

// DisplayBrightnessInfo contains "auto" or a percentage string from 0 through
// 100.
type DisplayBrightnessInfo struct {
	Value string `json:"value"`
}

// DisplayElements defines one application display update. Elements share the
// application name and priority; each element selects its own physical display.
type DisplayElements struct {
	ApplicationName      string           `json:"application_name"`
	Priority             int              `json:"priority,omitempty"`
	LEDNotificationColor string           `json:"led_notification_color,omitempty"`
	Elements             []DisplayElement `json:"elements"`
}

// DisplayElement is a supported element in a display update.
type DisplayElement interface {
	displayElement()
}

// BaseDisplayElement contains placement and lifetime fields shared by display
// elements. Positive Timeout and DisplayUntil values are mutually exclusive.
// Nil coordinates omit explicit placement on the corresponding axis.
type BaseDisplayElement struct {
	ID           string        `json:"id"`
	Timeout      *int          `json:"timeout,omitempty"`
	DisplayUntil string        `json:"display_until,omitempty"`
	X            *int          `json:"x,omitempty"`
	Y            *int          `json:"y,omitempty"`
	Display      DisplayTarget `json:"display,omitempty"`
	Align        DisplayAlign  `json:"align,omitempty"`
}

// DisplayTarget selects a physical device display.
type DisplayTarget = display.Target

const (
	// DisplayFront targets the front display.
	DisplayFront = display.Front
	// DisplayBack targets the back display.
	DisplayBack = display.Back
)

// DisplayAlign selects the anchor point for an element.
type DisplayAlign string

const (
	// DisplayAlignTopLeft anchors the element at the top left.
	DisplayAlignTopLeft DisplayAlign = "top_left"
	// DisplayAlignTopMid anchors the element at the top center.
	DisplayAlignTopMid DisplayAlign = "top_mid"
	// DisplayAlignTopRight anchors the element at the top right.
	DisplayAlignTopRight DisplayAlign = "top_right"
	// DisplayAlignMidLeft anchors the element at the middle left.
	DisplayAlignMidLeft DisplayAlign = "mid_left"
	// DisplayAlignCenter anchors the element at the center.
	DisplayAlignCenter DisplayAlign = "center"
	// DisplayAlignMidRight anchors the element at the middle right.
	DisplayAlignMidRight DisplayAlign = "mid_right"
	// DisplayAlignBottomLeft anchors the element at the bottom left.
	DisplayAlignBottomLeft DisplayAlign = "bottom_left"
	// DisplayAlignBottomMid anchors the element at the bottom center.
	DisplayAlignBottomMid DisplayAlign = "bottom_mid"
	// DisplayAlignBottomRight anchors the element at the bottom right.
	DisplayAlignBottomRight DisplayAlign = "bottom_right"
)

type displayElementType string

const (
	displayElementText      displayElementType = "text"
	displayElementImage     displayElementType = "image"
	displayElementAnimation displayElementType = "animation"
	displayElementCountdown displayElementType = "countdown"
	displayElementRectangle displayElementType = "rectangle"
)

// TextElement displays text with the selected font and scrolling behavior.
type TextElement struct {
	BaseDisplayElement
	Text              string `json:"text"`
	Font              Font   `json:"font"`
	Color             string `json:"color,omitempty"`
	Width             int    `json:"width,omitempty"`
	ScrollRate        int    `json:"scroll_rate,omitempty"`
	ScrollStartDelay  int    `json:"scroll_start_delay,omitempty"`
	ScrollRepeatDelay int    `json:"scroll_repeat_delay,omitempty"`
}

// Font selects a firmware-provided display font.
type Font string

const (
	// FontTiny selects the tiny font.
	FontTiny Font = "tiny"
	// FontSmall selects the small font.
	FontSmall Font = "small"
	// FontNormal selects the normal font.
	FontNormal Font = "normal"
	// FontCondensed selects the condensed font.
	FontCondensed Font = "condensed"
	// FontBold selects the bold font.
	FontBold Font = "bold"
	// FontLarge selects the large font.
	FontLarge Font = "large"
	// FontExtraLarge selects the extra-large font.
	FontExtraLarge Font = "extra_large"
	// FontGlobal selects the device global font.
	FontGlobal Font = "global"
)

// ImageElement displays a stored or stock image.
type ImageElement struct {
	BaseDisplayElement
	Path      string `json:"path,omitempty"`
	StockPath string `json:"stock_path,omitempty"`
	Opacity   *int   `json:"opacity,omitempty"`
}

// AnimationElement displays a stored or stock animation.
type AnimationElement struct {
	BaseDisplayElement
	Path             string `json:"path,omitempty"`
	StockPath        string `json:"stock_path,omitempty"`
	Loop             bool   `json:"loop,omitempty"`
	AwaitPreviousEnd bool   `json:"await_previous_end,omitempty"`
	Section          string `json:"section,omitempty"`
	Opacity          *int   `json:"opacity,omitempty"`
}

// CountdownElement displays elapsed or remaining time for a timestamp.
type CountdownElement struct {
	BaseDisplayElement
	Timestamp string             `json:"timestamp"`
	Color     string             `json:"color,omitempty"`
	Direction CountdownDirection `json:"direction"`
	ShowHours CountdownShowHours `json:"show_hours"`
}

// CountdownDirection selects elapsed or remaining time.
type CountdownDirection string

const (
	// CountdownTimeLeft shows time remaining until the timestamp.
	CountdownTimeLeft CountdownDirection = "time_left"
	// CountdownTimeSince shows time elapsed since the timestamp.
	CountdownTimeSince CountdownDirection = "time_since"
)

// CountdownShowHours controls when a countdown includes hours.
type CountdownShowHours string

const (
	// CountdownShowHoursWhenNonZero hides a zero hours field.
	CountdownShowHoursWhenNonZero CountdownShowHours = "when_non_zero"
	// CountdownShowHoursAlways always shows the hours field.
	CountdownShowHoursAlways CountdownShowHours = "always"
)

// RectangleElement displays a filled or outlined rectangle.
type RectangleElement struct {
	BaseDisplayElement
	Width       int           `json:"width"`
	Height      int           `json:"height"`
	Radius      int           `json:"radius,omitempty"`
	Fill        RectangleFill `json:"fill,omitempty"`
	FillColors  []string      `json:"fill_colors,omitempty"`
	BorderWidth *int          `json:"border_width,omitempty"`
	BorderColor string        `json:"border_color,omitempty"`
}

// RectangleFill selects the rectangle fill style.
type RectangleFill string

const (
	// RectangleFillNone leaves the rectangle transparent.
	RectangleFillNone RectangleFill = "none"
	// RectangleFillSolid uses one fill color.
	RectangleFillSolid RectangleFill = "solid"
	// RectangleFillGradientH uses a horizontal gradient.
	RectangleFillGradientH RectangleFill = "gradient_h"
	// RectangleFillGradientV uses a vertical gradient.
	RectangleFillGradientV RectangleFill = "gradient_v"
)

func (TextElement) displayElement()      {}
func (ImageElement) displayElement()     {}
func (AnimationElement) displayElement() {}
func (CountdownElement) displayElement() {}
func (RectangleElement) displayElement() {}

// MarshalJSON adds the text element wire discriminator.
func (e TextElement) MarshalJSON() ([]byte, error) {
	type alias TextElement
	return json.Marshal(struct {
		Type displayElementType `json:"type"`
		alias
	}{Type: displayElementText, alias: alias(e)})
}

// MarshalJSON adds the image element wire discriminator.
func (e ImageElement) MarshalJSON() ([]byte, error) {
	type alias ImageElement
	return json.Marshal(struct {
		Type displayElementType `json:"type"`
		alias
	}{Type: displayElementImage, alias: alias(e)})
}

// MarshalJSON adds the animation element wire discriminator.
func (e AnimationElement) MarshalJSON() ([]byte, error) {
	type alias AnimationElement
	return json.Marshal(struct {
		Type displayElementType `json:"type"`
		alias
	}{Type: displayElementAnimation, alias: alias(e)})
}

// MarshalJSON adds the countdown element wire discriminator.
func (e CountdownElement) MarshalJSON() ([]byte, error) {
	type alias CountdownElement
	return json.Marshal(struct {
		Type displayElementType `json:"type"`
		alias
	}{Type: displayElementCountdown, alias: alias(e)})
}

// MarshalJSON adds the rectangle element wire discriminator.
func (e RectangleElement) MarshalJSON() ([]byte, error) {
	type alias RectangleElement
	return json.Marshal(struct {
		Type displayElementType `json:"type"`
		alias
	}{Type: displayElementRectangle, alias: alias(e)})
}

// PlayAudio selects exactly one uploaded Path or firmware StockPath for
// playback.
type PlayAudio struct {
	ApplicationName string `json:"application_name"`
	Path            string `json:"path,omitempty"`
	StockPath       string `json:"stock_path,omitempty"`
}

// AudioVolumeInfo contains the current playback volume from 0 through 100.
type AudioVolumeInfo struct {
	Volume int `json:"volume"`
}

// SetAudioVolumeRequest changes playback volume from 0 through 100. Silent
// suppresses the device's feedback sound.
type SetAudioVolumeRequest struct {
	Volume int
	Silent bool
}

// UploadAssetRequest stores Body as File under ApplicationName.
type UploadAssetRequest struct {
	ApplicationName string
	File            string
	Body            Body
}

// WriteStorageFileRequest writes a body to a device storage path.
type WriteStorageFileRequest struct {
	Path string
	Body Body
}

// StorageList contains the entries in a device storage directory.
type StorageList struct {
	List []StorageListElement `json:"list"`
}

// StorageListElement describes one file or directory.
type StorageListElement struct {
	Type StorageListElementType `json:"type"`
	Name string                 `json:"name"`
	Size uint64                 `json:"size,omitempty"`
}

// StorageListElementType identifies a file-system entry type.
type StorageListElementType string

const (
	// StorageListElementFile identifies a regular file.
	StorageListElementFile StorageListElementType = "file"
	// StorageListElementDir identifies a directory.
	StorageListElementDir StorageListElementType = "dir"
)

// StorageStatus reports device storage capacity and use.
type StorageStatus struct {
	UsedBytes  uint64 `json:"used_bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
	TotalBytes uint64 `json:"total_bytes"`
}

// BusySnapshot contains the complete current timer state and its device
// timestamp in milliseconds.
type BusySnapshot struct {
	Snapshot            BusySnapshotData `json:"snapshot"`
	SnapshotTimestampMS int64            `json:"snapshot_timestamp_ms"`
}

// BusySnapshotData describes the active timer and Busy Bar settings. Required
// pointer fields depend on Type; Validate checks the complete combination.
type BusySnapshotData struct {
	Type                       BusySnapshotType           `json:"type"`
	CardID                     string                     `json:"card_id,omitempty"`
	TimeLeftMS                 *int64                     `json:"time_left_ms,omitempty"`
	IsPaused                   *bool                      `json:"is_paused,omitempty"`
	CurrentInterval            *int                       `json:"current_interval,omitempty"`
	CurrentIntervalTimeTotalMS *int64                     `json:"current_interval_time_total_ms,omitempty"`
	CurrentIntervalTimeLeftMS  *int64                     `json:"current_interval_time_left_ms,omitempty"`
	IntervalSettings           *BusyTimerIntervalSettings `json:"interval_settings,omitempty"`
	BusyBarSettings            BusyBarSettings            `json:"busy_bar_settings"`
}

// BusySnapshotType identifies the active timer mode.
type BusySnapshotType string

const (
	// BusySnapshotNotStarted means no timer has started.
	BusySnapshotNotStarted BusySnapshotType = "NOT_STARTED"
	// BusySnapshotInfinite means an infinite timer is active.
	BusySnapshotInfinite BusySnapshotType = "INFINITE"
	// BusySnapshotSimple means a fixed-duration timer is active.
	BusySnapshotSimple BusySnapshotType = "SIMPLE"
	// BusySnapshotInterval means an interval timer is active.
	BusySnapshotInterval BusySnapshotType = "INTERVAL"
)

// BusyProfile defines the complete contents of a saved timer profile.
type BusyProfile struct {
	SortOrder          int               `json:"sort_order"`
	Title              string            `json:"title"`
	ID                 string            `json:"id"`
	TimerSettings      BusyTimerSettings `json:"timer_settings"`
	BusyBarSettings    BusyBarSettings   `json:"busy_bar_settings"`
	ProfileTimestampMS int64             `json:"profile_timestamp_ms"`
}

// BusyProfileSlot selects a built-in profile slot.
type BusyProfileSlot string

const (
	// BusyProfileSlotBusy selects the Busy button profile.
	BusyProfileSlotBusy BusyProfileSlot = "busy"
	// BusyProfileSlotCustom selects the Custom button profile.
	BusyProfileSlotCustom BusyProfileSlot = "custom"
)

// BusyTimerSettings configures an infinite, simple, or interval timer. Required
// pointer fields depend on Type; duration fields use milliseconds.
type BusyTimerSettings struct {
	Type                    BusyTimerType `json:"type"`
	TotalTimeMS             *int64        `json:"total_time_ms,omitempty"`
	IntervalWorkMS          *int64        `json:"interval_work_ms,omitempty"`
	IntervalRestMS          *int64        `json:"interval_rest_ms,omitempty"`
	IntervalWorkCyclesCount *int          `json:"interval_work_cycles_count,omitempty"`
	IsAutostartEnabled      *bool         `json:"is_autostart_enabled,omitempty"`
}

// BusyTimerType identifies a timer mode.
type BusyTimerType string

const (
	// BusyTimerInfinite runs until the user stops it.
	BusyTimerInfinite BusyTimerType = "INFINITE"
	// BusyTimerSimple runs for one fixed duration.
	BusyTimerSimple BusyTimerType = "SIMPLE"
	// BusyTimerInterval alternates work and rest periods.
	BusyTimerInterval BusyTimerType = "INTERVAL"
)

// BusyTimerIntervalSettings describes the active interval timer.
type BusyTimerIntervalSettings struct {
	Type                    BusyTimerType `json:"type"`
	IntervalWorkMS          int64         `json:"interval_work_ms"`
	IntervalRestMS          int64         `json:"interval_rest_ms"`
	IntervalWorkCyclesCount int           `json:"interval_work_cycles_count"`
	IsAutostartEnabled      bool          `json:"is_autostart_enabled"`
}

// BusyBarSettings controls timer display and smart-home behavior.
type BusyBarSettings struct {
	Theme             string `json:"theme"`
	ShowWorkPhaseOnly bool   `json:"show_work_phase_only"`
	TriggerSmartHome  bool   `json:"trigger_smart_home"`
}

// AccountLink contains a temporary account-link code and its firmware-provided
// expiry time. Treat Code as authorization data.
type AccountLink struct {
	Code      string `json:"code"`
	ExpiresAt int64  `json:"expires_at"`
}

// AccountInfo reports the account linked to the device. Email and user IDs can
// contain private account data.
type AccountInfo struct {
	Linked bool   `json:"linked"`
	ID     string `json:"id"`
	Email  string `json:"email"`
	UserID string `json:"user_id"`
}

// AccountStatus reports the connection state of the linked account.
type AccountStatus struct {
	Status AccountConnectionStatus `json:"status"`
}

// AccountConnectionStatus identifies the account backend connection state.
type AccountConnectionStatus string

const (
	// AccountStatusError means the account connection failed.
	AccountStatusError AccountConnectionStatus = "error"
	// AccountStatusDisconnected means no account connection is active.
	AccountStatusDisconnected AccountConnectionStatus = "disconnected"
	// AccountStatusConnected means the account connection is active.
	AccountStatusConnected AccountConnectionStatus = "connected"
)

// AccountBackend configures the account server and client certificate mode.
// IgnoreServerCert asks the firmware to ignore server-certificate validation;
// use it only with an explicitly trusted development endpoint.
type AccountBackend struct {
	ServerURL        string                `json:"server_url"`
	ClientCertType   AccountClientCertType `json:"client_cert_type"`
	IgnoreServerCert bool                  `json:"ignore_server_cert"`
}

// AccountClientCertType selects the certificate used for account requests.
type AccountClientCertType string

const (
	// AccountClientCertDefault uses the firmware default certificate.
	AccountClientCertDefault AccountClientCertType = "default"
	// AccountClientCertCustom uses a user-provided certificate.
	AccountClientCertCustom AccountClientCertType = "custom"
	// AccountClientCertNone sends no client certificate.
	AccountClientCertNone AccountClientCertType = "none"
)

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

// WiFiStatus reports the current Wi-Fi connection and network settings.
type WiFiStatus struct {
	State    WiFiConnectionState `json:"state"`
	SSID     string              `json:"ssid,omitempty"`
	BSSID    string              `json:"bssid,omitempty"`
	Channel  int                 `json:"channel,omitempty"`
	RSSI     int                 `json:"rssi,omitempty"`
	Security WiFiSecurityMethod  `json:"security,omitempty"`
	IPConfig *WiFiIPConfig       `json:"ip_config,omitempty"`
}

// WiFiConnectionState identifies the Wi-Fi connection lifecycle state.
type WiFiConnectionState string

const (
	// WiFiStateUnknown means the firmware cannot determine the Wi-Fi state.
	WiFiStateUnknown WiFiConnectionState = "unknown"
	// WiFiStateDisconnected means no Wi-Fi network is connected.
	WiFiStateDisconnected WiFiConnectionState = "disconnected"
	// WiFiStateConnected means a Wi-Fi network is connected.
	WiFiStateConnected WiFiConnectionState = "connected"
	// WiFiStateConnecting means a connection attempt is in progress.
	WiFiStateConnecting WiFiConnectionState = "connecting"
	// WiFiStateDisconnecting means disconnection is in progress.
	WiFiStateDisconnecting WiFiConnectionState = "disconnecting"
	// WiFiStateReconnecting means a reconnection attempt is in progress.
	WiFiStateReconnecting WiFiConnectionState = "reconnecting"
)

// WiFiNetworkList contains the Wi-Fi networks found by a scan.
type WiFiNetworkList struct {
	Count    int           `json:"count"`
	Networks []WiFiNetwork `json:"networks"`
}

// WiFiNetwork describes one Wi-Fi network found by a scan.
type WiFiNetwork struct {
	SSID     string             `json:"ssid"`
	Security WiFiSecurityMethod `json:"security"`
	RSSI     int                `json:"rssi"`
}

// WiFiConnectRequest configures a Wi-Fi connection request. Password contains
// network credentials and should not be logged.
type WiFiConnectRequest struct {
	SSID     string              `json:"ssid"`
	Password string              `json:"password"`
	Security WiFiSecurityMethod  `json:"security"`
	IPConfig WiFiConnectIPConfig `json:"ip_config"`
}

// WiFiConnectIPConfig configures DHCP or a static IPv4 address.
type WiFiConnectIPConfig struct {
	IPMethod WiFiIPMethod `json:"ip_method"`
	Address  string       `json:"address,omitempty"`
	Mask     string       `json:"mask,omitempty"`
	Gateway  string       `json:"gateway,omitempty"`
}

// WiFiSecurityMethod identifies a firmware-supported Wi-Fi security mode.
type WiFiSecurityMethod string

const (
	// WiFiSecurityOpen selects an unsecured network.
	WiFiSecurityOpen WiFiSecurityMethod = "Open"
	// WiFiSecurityWPA selects WPA security.
	WiFiSecurityWPA WiFiSecurityMethod = "WPA"
	// WiFiSecurityWPA2 selects WPA2 security.
	WiFiSecurityWPA2 WiFiSecurityMethod = "WPA2"
	// WiFiSecurityWEP selects WEP security.
	WiFiSecurityWEP WiFiSecurityMethod = "WEP"
	// WiFiSecurityWPAWPA2 selects mixed WPA and WPA2 security.
	WiFiSecurityWPAWPA2 WiFiSecurityMethod = "WPA/WPA2"
	// WiFiSecurityWPA3 selects WPA3 security.
	WiFiSecurityWPA3 WiFiSecurityMethod = "WPA3"
	// WiFiSecurityWPA2WPA3 selects mixed WPA2 and WPA3 security.
	WiFiSecurityWPA2WPA3 WiFiSecurityMethod = "WPA2/WPA3"
	// WiFiSecurityUnsupported identifies an unsupported security mode.
	WiFiSecurityUnsupported WiFiSecurityMethod = "Unsupported"
)

// WiFiIPConfig reports the current Wi-Fi IP configuration.
type WiFiIPConfig struct {
	IPMethod WiFiIPMethod `json:"ip_method,omitempty"`
	IPType   WiFiIPType   `json:"ip_type,omitempty"`
	Address  string       `json:"address,omitempty"`
}

// WiFiIPMethod identifies dynamic or static address assignment.
type WiFiIPMethod string

const (
	// WiFiIPMethodDHCP requests dynamic address assignment.
	WiFiIPMethodDHCP WiFiIPMethod = "dhcp"
	// WiFiIPMethodStatic uses caller-provided address settings.
	WiFiIPMethodStatic WiFiIPMethod = "static"
)

// WiFiIPType identifies the address protocol version.
type WiFiIPType string

const (
	// WiFiIPTypeIPv4 identifies an IPv4 address.
	WiFiIPTypeIPv4 WiFiIPType = "ipv4"
	// WiFiIPTypeIPv6 identifies an IPv6 address.
	WiFiIPTypeIPv6 WiFiIPType = "ipv6"
)

// InputKey identifies a physical or virtual device key.
type InputKey string

const (
	// InputKeyUp sends the Up key.
	InputKeyUp InputKey = "up"
	// InputKeyDown sends the Down key.
	InputKeyDown InputKey = "down"
	// InputKeyOK sends the OK key.
	InputKeyOK InputKey = "ok"
	// InputKeyBack sends the Back key.
	InputKeyBack InputKey = "back"
	// InputKeyStart sends the Start key.
	InputKeyStart InputKey = "start"
	// InputKeyBusy sends the Busy key.
	InputKeyBusy InputKey = "busy"
	// InputKeyCustom sends the Custom key.
	InputKeyCustom InputKey = "custom"
	// InputKeyOff sends the Off key.
	InputKeyOff InputKey = "off"
	// InputKeyApps sends the Apps key.
	InputKeyApps InputKey = "apps"
	// InputKeySettings sends the Settings key.
	InputKeySettings InputKey = "settings"
)

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

// TimestampInfo contains an RFC 3339 device timestamp.
type TimestampInfo struct {
	Timestamp string `json:"timestamp"`
}

// TimezoneInfo describes a firmware-supported time zone.
type TimezoneInfo struct {
	Name   string `json:"name"`
	Offset string `json:"offset"`
	Abbr   string `json:"abbr"`
}

// TimezoneListResponse contains the firmware-supported time zones.
type TimezoneListResponse struct {
	List []TimezoneInfo `json:"list"`
}

// UpdateStatus reports firmware update installation and availability checks.
type UpdateStatus struct {
	Install UpdateInstallStatus `json:"install"`
	Check   UpdateCheckStatus   `json:"check"`
}

// UpdateInstallStatus reports the current firmware installation step and result.
type UpdateInstallStatus struct {
	IsAllowed bool                 `json:"is_allowed"`
	Event     UpdateInstallEvent   `json:"event"`
	Action    UpdateInstallAction  `json:"action"`
	Status    UpdateInstallResult  `json:"status"`
	Detail    string               `json:"detail,omitempty"`
	Download  UpdateDownloadStatus `json:"download"`
}

// UpdateDownloadStatus reports firmware download progress.
type UpdateDownloadStatus struct {
	SpeedBytesPerSec int64 `json:"speed_bytes_per_sec"`
	ReceivedBytes    int64 `json:"received_bytes"`
	TotalBytes       int64 `json:"total_bytes"`
}

// UpdateCheckStatus reports the latest firmware availability check.
type UpdateCheckStatus struct {
	AvailableVersion string            `json:"available_version"`
	Event            UpdateCheckEvent  `json:"event"`
	Status           UpdateCheckResult `json:"status"`
}

// UpdateInstallEvent identifies a change in the installation lifecycle.
type UpdateInstallEvent string

const (
	// UpdateInstallEventSessionStart begins an installation session.
	UpdateInstallEventSessionStart UpdateInstallEvent = "session_start"
	// UpdateInstallEventSessionStop ends an installation session.
	UpdateInstallEventSessionStop UpdateInstallEvent = "session_stop"
	// UpdateInstallEventActionBegin begins one installation action.
	UpdateInstallEventActionBegin UpdateInstallEvent = "action_begin"
	// UpdateInstallEventActionDone completes one installation action.
	UpdateInstallEventActionDone UpdateInstallEvent = "action_done"
	// UpdateInstallEventDetailChange updates installation detail text.
	UpdateInstallEventDetailChange UpdateInstallEvent = "detail_change"
	// UpdateInstallEventActionProgress updates installation progress.
	UpdateInstallEventActionProgress UpdateInstallEvent = "action_progress"
	// UpdateInstallEventNone means no installation event is active.
	UpdateInstallEventNone UpdateInstallEvent = "none"
)

// UpdateInstallAction identifies the active firmware installation step.
type UpdateInstallAction string

const (
	// UpdateInstallActionDownload downloads the firmware package.
	UpdateInstallActionDownload UpdateInstallAction = "download"
	// UpdateInstallActionSHAVerification verifies the package digest.
	UpdateInstallActionSHAVerification UpdateInstallAction = "sha_verification"
	// UpdateInstallActionUnpack extracts the firmware package.
	UpdateInstallActionUnpack UpdateInstallAction = "unpack"
	// UpdateInstallActionPrepare prepares the installation target.
	UpdateInstallActionPrepare UpdateInstallAction = "prepare"
	// UpdateInstallActionApply applies the prepared firmware.
	UpdateInstallActionApply UpdateInstallAction = "apply"
	// UpdateInstallActionNone means no installation action is active.
	UpdateInstallActionNone UpdateInstallAction = "none"
)

// UpdateInstallResult identifies the firmware installation result.
type UpdateInstallResult string

const (
	// UpdateInstallOK means installation succeeded.
	UpdateInstallOK UpdateInstallResult = "ok"
	// UpdateInstallBatteryLow means the battery charge is too low.
	UpdateInstallBatteryLow UpdateInstallResult = "battery_low"
	// UpdateInstallBusy means another operation blocks installation.
	UpdateInstallBusy UpdateInstallResult = "busy"
	// UpdateInstallDownloadFailure means the package download failed.
	UpdateInstallDownloadFailure UpdateInstallResult = "download_failure"
	// UpdateInstallDownloadAbort means the package download was canceled.
	UpdateInstallDownloadAbort UpdateInstallResult = "download_abort"
	// UpdateInstallSHAMismatch means package digest verification failed.
	UpdateInstallSHAMismatch UpdateInstallResult = "sha_mismatch"
	// UpdateInstallUnpackStagingDirFailure means staging directory creation failed.
	UpdateInstallUnpackStagingDirFailure UpdateInstallResult = "unpack_staging_dir_failure"
	// UpdateInstallUnpackArchiveOpenFailure means the firmware archive could not open.
	UpdateInstallUnpackArchiveOpenFailure UpdateInstallResult = "unpack_archive_open_failure"
	// UpdateInstallUnpackArchiveUnpackFailure means archive extraction failed.
	UpdateInstallUnpackArchiveUnpackFailure UpdateInstallResult = "unpack_archive_unpack_failure"
	// UpdateInstallManifestNotFound means the package has no install manifest.
	UpdateInstallManifestNotFound UpdateInstallResult = "install_manifest_not_found"
	// UpdateInstallManifestInvalid means the install manifest is invalid.
	UpdateInstallManifestInvalid UpdateInstallResult = "install_manifest_invalid"
	// UpdateInstallSessionConfigFailure means session configuration failed.
	UpdateInstallSessionConfigFailure UpdateInstallResult = "install_session_config_failure"
	// UpdateInstallPointerSetupFailure means installation pointer setup failed.
	UpdateInstallPointerSetupFailure UpdateInstallResult = "install_pointer_setup_failure"
	// UpdateInstallUnknownFailure means installation failed for an unknown reason.
	UpdateInstallUnknownFailure UpdateInstallResult = "unknown_failure"
)

// UpdateCheckEvent identifies a firmware availability check lifecycle event.
type UpdateCheckEvent string

const (
	// UpdateCheckEventStart begins an availability check.
	UpdateCheckEventStart UpdateCheckEvent = "start"
	// UpdateCheckEventStop ends an availability check.
	UpdateCheckEventStop UpdateCheckEvent = "stop"
	// UpdateCheckEventNone means no availability check event is active.
	UpdateCheckEventNone UpdateCheckEvent = "none"
)

// UpdateCheckResult identifies the result of a firmware availability check.
type UpdateCheckResult string

const (
	// UpdateCheckAvailable means a firmware update is available.
	UpdateCheckAvailable UpdateCheckResult = "available"
	// UpdateCheckNotAvailable means the current firmware is up to date.
	UpdateCheckNotAvailable UpdateCheckResult = "not_available"
	// UpdateCheckFailure means the availability check failed.
	UpdateCheckFailure UpdateCheckResult = "failure"
	// UpdateCheckNone means no availability check result exists.
	UpdateCheckNone UpdateCheckResult = "none"
)

// UpdateChangelog contains the changelog for an available firmware update.
type UpdateChangelog struct {
	Changelog string `json:"changelog"`
}

// AutoUpdateSettings configures automatic updates and their daily time window.
// IntervalStart and IntervalEnd use 24-hour HH:MM values. Nil IsEnabled leaves
// the current enabled state unchanged when updating settings.
type AutoUpdateSettings struct {
	IsEnabled     *bool  `json:"is_enabled,omitempty"`
	IntervalStart string `json:"interval_start,omitempty"`
	IntervalEnd   string `json:"interval_end,omitempty"`
}
