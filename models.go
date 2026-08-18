package busylib

import "encoding/json"

type SuccessResponse struct {
	Result string `json:"result"`
}

type LogDumpResponse struct {
	Result string `json:"result"`
	Path   string `json:"path"`
}

type VersionInfo struct {
	APISemVer string `json:"api_semver"`
}

type NetworkInterfaceInfo struct {
	Type TransportType `json:"type"`
}

type TransportType string

const (
	TransportUSB  TransportType = "usb"
	TransportWiFi TransportType = "wifi"
)

type Status struct {
	Device   StatusDevice   `json:"device"`
	Firmware StatusFirmware `json:"firmware"`
	System   StatusSystem   `json:"system"`
	Power    StatusPower    `json:"power"`
}

type StatusDevice struct {
	SerialNumber     string `json:"serial_number"`
	USBMAC           string `json:"usb_mac"`
	WiFiMAC          string `json:"wifi_mac,omitempty"`
	BLEMAC           string `json:"ble_mac,omitempty"`
	OTPValid         bool   `json:"otp_valid"`
	OTPModel         string `json:"otp_model,omitempty"`
	OTPTimestamp     int64  `json:"otp_timestamp,omitempty"`
	FirmwareSecurity string `json:"firmware_security"`
}

type StatusFirmware struct {
	Version         string `json:"version"`
	Target          int    `json:"target"`
	Branch          string `json:"branch"`
	BuildDate       string `json:"build_date"`
	CommitHash      string `json:"commit_hash"`
	IntercomVersion string `json:"intercom_version"`
	NWPVersion      string `json:"nwp_version,omitempty"`
	MatterVersion   string `json:"matter_version,omitempty"`
}

type StatusSystem struct {
	APISemVer         string `json:"api_semver"`
	Uptime            string `json:"uptime"`
	BootTime          int64  `json:"boot_time"`
	AutoUpdateEnabled bool   `json:"auto_update_enabled"`
}

type StatusPower struct {
	State          PowerState `json:"state"`
	BatteryCharge  int        `json:"battery_charge"`
	BatteryVoltage int        `json:"battery_voltage"`
	BatteryCurrent int        `json:"battery_current"`
	USBVoltage     int        `json:"usb_voltage"`
}

type PowerState string

const (
	PowerDischarging PowerState = "discharging"
	PowerCharging    PowerState = "charging"
	PowerCharged     PowerState = "charged"
)

type HttpAccessInfo struct {
	Mode     HTTPAccessMode `json:"mode"`
	KeyValid bool           `json:"key_valid"`
}

type HTTPAccessMode string

const (
	HTTPAccessDisabled HTTPAccessMode = "disabled"
	HTTPAccessEnabled  HTTPAccessMode = "enabled"
	HTTPAccessKey      HTTPAccessMode = "key"
)

type NameInfo struct {
	Name string `json:"name"`
}

type DisplayBrightnessInfo struct {
	Value string `json:"value"`
}

type DisplayElements struct {
	ApplicationName      string           `json:"application_name"`
	Priority             int              `json:"priority,omitempty"`
	LEDNotificationColor string           `json:"led_notification_color,omitempty"`
	Elements             []DisplayElement `json:"elements"`
}

type DisplayElement interface {
	displayElement()
}

type BaseDisplayElement struct {
	ID           string        `json:"id"`
	Timeout      *int          `json:"timeout,omitempty"`
	DisplayUntil string        `json:"display_until,omitempty"`
	X            *int          `json:"x,omitempty"`
	Y            *int          `json:"y,omitempty"`
	Display      DisplayTarget `json:"display,omitempty"`
	Align        DisplayAlign  `json:"align,omitempty"`
}

type DisplayTarget string

const (
	DisplayFront DisplayTarget = "front"
	DisplayBack  DisplayTarget = "back"
)

type DisplayAlign string

const (
	DisplayAlignTopLeft     DisplayAlign = "top_left"
	DisplayAlignTopMid      DisplayAlign = "top_mid"
	DisplayAlignTopRight    DisplayAlign = "top_right"
	DisplayAlignMidLeft     DisplayAlign = "mid_left"
	DisplayAlignCenter      DisplayAlign = "center"
	DisplayAlignMidRight    DisplayAlign = "mid_right"
	DisplayAlignBottomLeft  DisplayAlign = "bottom_left"
	DisplayAlignBottomMid   DisplayAlign = "bottom_mid"
	DisplayAlignBottomRight DisplayAlign = "bottom_right"
)

type DisplayElementType string

const (
	DisplayElementText      DisplayElementType = "text"
	DisplayElementImage     DisplayElementType = "image"
	DisplayElementAnimation DisplayElementType = "animation"
	DisplayElementCountdown DisplayElementType = "countdown"
	DisplayElementRectangle DisplayElementType = "rectangle"
)

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

type Font string

const (
	FontTiny       Font = "tiny"
	FontSmall      Font = "small"
	FontNormal     Font = "normal"
	FontCondensed  Font = "condensed"
	FontBold       Font = "bold"
	FontLarge      Font = "large"
	FontExtraLarge Font = "extra_large"
	FontGlobal     Font = "global"
)

type ImageElement struct {
	BaseDisplayElement
	Path      string `json:"path,omitempty"`
	StockPath string `json:"stock_path,omitempty"`
	Opacity   *int   `json:"opacity,omitempty"`
}

type AnimationElement struct {
	BaseDisplayElement
	Path             string `json:"path,omitempty"`
	StockPath        string `json:"stock_path,omitempty"`
	Loop             bool   `json:"loop,omitempty"`
	AwaitPreviousEnd bool   `json:"await_previous_end,omitempty"`
	Section          string `json:"section,omitempty"`
	Opacity          *int   `json:"opacity,omitempty"`
}

type CountdownElement struct {
	BaseDisplayElement
	Timestamp string             `json:"timestamp"`
	Color     string             `json:"color,omitempty"`
	Direction CountdownDirection `json:"direction"`
	ShowHours CountdownShowHours `json:"show_hours"`
}

type CountdownDirection string

const (
	CountdownTimeLeft  CountdownDirection = "time_left"
	CountdownTimeSince CountdownDirection = "time_since"
)

type CountdownShowHours string

const (
	CountdownShowHoursWhenNonZero CountdownShowHours = "when_non_zero"
	CountdownShowHoursAlways      CountdownShowHours = "always"
)

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

type RectangleFill string

const (
	RectangleFillNone      RectangleFill = "none"
	RectangleFillSolid     RectangleFill = "solid"
	RectangleFillGradientH RectangleFill = "gradient_h"
	RectangleFillGradientV RectangleFill = "gradient_v"
)

func (TextElement) displayElement()      {}
func (ImageElement) displayElement()     {}
func (AnimationElement) displayElement() {}
func (CountdownElement) displayElement() {}
func (RectangleElement) displayElement() {}

func (e TextElement) MarshalJSON() ([]byte, error) {
	type alias TextElement
	return json.Marshal(struct {
		Type DisplayElementType `json:"type"`
		alias
	}{Type: DisplayElementText, alias: alias(e)})
}

func (e ImageElement) MarshalJSON() ([]byte, error) {
	type alias ImageElement
	return json.Marshal(struct {
		Type DisplayElementType `json:"type"`
		alias
	}{Type: DisplayElementImage, alias: alias(e)})
}

func (e AnimationElement) MarshalJSON() ([]byte, error) {
	type alias AnimationElement
	return json.Marshal(struct {
		Type DisplayElementType `json:"type"`
		alias
	}{Type: DisplayElementAnimation, alias: alias(e)})
}

func (e CountdownElement) MarshalJSON() ([]byte, error) {
	type alias CountdownElement
	return json.Marshal(struct {
		Type DisplayElementType `json:"type"`
		alias
	}{Type: DisplayElementCountdown, alias: alias(e)})
}

func (e RectangleElement) MarshalJSON() ([]byte, error) {
	type alias RectangleElement
	return json.Marshal(struct {
		Type DisplayElementType `json:"type"`
		alias
	}{Type: DisplayElementRectangle, alias: alias(e)})
}

type PlayAudio struct {
	ApplicationName string `json:"application_name"`
	Path            string `json:"path,omitempty"`
	StockPath       string `json:"stock_path,omitempty"`
}

type AudioVolumeInfo struct {
	Volume int `json:"volume"`
}

type SetAudioVolumeRequest struct {
	Volume int
	Silent bool
}

type UploadAssetRequest struct {
	ApplicationName string
	File            string
	Body            Body
}

type WriteStorageFileRequest struct {
	Path string
	Body Body
}

type StorageList struct {
	List []StorageListElement `json:"list"`
}

type StorageListElement struct {
	Type StorageListElementType `json:"type"`
	Name string                 `json:"name"`
	Size uint64                 `json:"size,omitempty"`
}

type StorageListElementType string

const (
	StorageListElementFile StorageListElementType = "file"
	StorageListElementDir  StorageListElementType = "dir"
)

type StorageStatus struct {
	UsedBytes  uint64 `json:"used_bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
	TotalBytes uint64 `json:"total_bytes"`
}

type BusySnapshot struct {
	Snapshot            BusySnapshotData `json:"snapshot"`
	SnapshotTimestampMS int64            `json:"snapshot_timestamp_ms"`
}

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

type BusySnapshotType string

const (
	BusySnapshotNotStarted BusySnapshotType = "NOT_STARTED"
	BusySnapshotInfinite   BusySnapshotType = "INFINITE"
	BusySnapshotSimple     BusySnapshotType = "SIMPLE"
	BusySnapshotInterval   BusySnapshotType = "INTERVAL"
)

type BusyProfile struct {
	SortOrder          int               `json:"sort_order"`
	Title              string            `json:"title"`
	ID                 string            `json:"id"`
	TimerSettings      BusyTimerSettings `json:"timer_settings"`
	BusyBarSettings    BusyBarSettings   `json:"busy_bar_settings"`
	ProfileTimestampMS int64             `json:"profile_timestamp_ms"`
}

type BusyProfileSlot string

const (
	BusyProfileSlotBusy   BusyProfileSlot = "busy"
	BusyProfileSlotCustom BusyProfileSlot = "custom"
)

type BusyTimerSettings struct {
	Type                    BusyTimerType `json:"type"`
	TotalTimeMS             *int64        `json:"total_time_ms,omitempty"`
	IntervalWorkMS          *int64        `json:"interval_work_ms,omitempty"`
	IntervalRestMS          *int64        `json:"interval_rest_ms,omitempty"`
	IntervalWorkCyclesCount *int          `json:"interval_work_cycles_count,omitempty"`
	IsAutostartEnabled      *bool         `json:"is_autostart_enabled,omitempty"`
}

type BusyTimerType string

const (
	BusyTimerInfinite BusyTimerType = "INFINITE"
	BusyTimerSimple   BusyTimerType = "SIMPLE"
	BusyTimerInterval BusyTimerType = "INTERVAL"
)

type BusyTimerIntervalSettings struct {
	Type                    BusyTimerType `json:"type"`
	IntervalWorkMS          int64         `json:"interval_work_ms"`
	IntervalRestMS          int64         `json:"interval_rest_ms"`
	IntervalWorkCyclesCount int           `json:"interval_work_cycles_count"`
	IsAutostartEnabled      bool          `json:"is_autostart_enabled"`
}

type BusyBarSettings struct {
	Theme             string `json:"theme"`
	ShowWorkPhaseOnly bool   `json:"show_work_phase_only"`
	TriggerSmartHome  bool   `json:"trigger_smart_home"`
}

type AccountLink struct {
	Code      string `json:"code"`
	ExpiresAt int64  `json:"expires_at"`
}

type AccountInfo struct {
	Linked bool   `json:"linked"`
	ID     string `json:"id"`
	Email  string `json:"email"`
	UserID string `json:"user_id"`
}

type AccountStatus struct {
	Status AccountConnectionStatus `json:"status"`
}

type AccountConnectionStatus string

const (
	AccountStatusError        AccountConnectionStatus = "error"
	AccountStatusDisconnected AccountConnectionStatus = "disconnected"
	AccountStatusConnected    AccountConnectionStatus = "connected"
)

type AccountBackend struct {
	ServerURL        string                `json:"server_url"`
	ClientCertType   AccountClientCertType `json:"client_cert_type"`
	IgnoreServerCert bool                  `json:"ignore_server_cert"`
}

type AccountClientCertType string

const (
	AccountClientCertDefault AccountClientCertType = "default"
	AccountClientCertCustom  AccountClientCertType = "custom"
	AccountClientCertNone    AccountClientCertType = "none"
)

type BleStatusResponse struct {
	Status  BLEStatus `json:"status"`
	Address string    `json:"address,omitempty"`
}

type BLEStatus string

const (
	BLEStatusReset          BLEStatus = "reset"
	BLEStatusInitialization BLEStatus = "initialization"
	BLEStatusDisabled       BLEStatus = "disabled"
	BLEStatusEnabled        BLEStatus = "enabled"
	BLEStatusConnectable    BLEStatus = "connectable"
	BLEStatusConnected      BLEStatus = "connected"
	BLEStatusInternalError  BLEStatus = "internal error"
)

type WiFiStatus struct {
	State    WiFiConnectionState `json:"state"`
	SSID     string              `json:"ssid,omitempty"`
	BSSID    string              `json:"bssid,omitempty"`
	Channel  int                 `json:"channel,omitempty"`
	RSSI     int                 `json:"rssi,omitempty"`
	Security WiFiSecurityMethod  `json:"security,omitempty"`
	IPConfig *WiFiIPConfig       `json:"ip_config,omitempty"`
}

type WiFiConnectionState string

const (
	WiFiStateUnknown       WiFiConnectionState = "unknown"
	WiFiStateDisconnected  WiFiConnectionState = "disconnected"
	WiFiStateConnected     WiFiConnectionState = "connected"
	WiFiStateConnecting    WiFiConnectionState = "connecting"
	WiFiStateDisconnecting WiFiConnectionState = "disconnecting"
	WiFiStateReconnecting  WiFiConnectionState = "reconnecting"
)

type NetworkResponse struct {
	Count    int       `json:"count"`
	Networks []Network `json:"networks"`
}

type Network struct {
	SSID     string             `json:"ssid"`
	Security WiFiSecurityMethod `json:"security"`
	RSSI     int                `json:"rssi"`
}

type ConnectRequestConfig struct {
	SSID     string              `json:"ssid"`
	Password string              `json:"password"`
	Security WiFiSecurityMethod  `json:"security"`
	IPConfig WiFiConnectIPConfig `json:"ip_config"`
}

type WiFiConnectIPConfig struct {
	IPMethod WiFiIPMethod `json:"ip_method"`
	Address  string       `json:"address,omitempty"`
	Mask     string       `json:"mask,omitempty"`
	Gateway  string       `json:"gateway,omitempty"`
}

type WiFiSecurityMethod string

const (
	WiFiSecurityOpen        WiFiSecurityMethod = "Open"
	WiFiSecurityWPA         WiFiSecurityMethod = "WPA"
	WiFiSecurityWPA2        WiFiSecurityMethod = "WPA2"
	WiFiSecurityWEP         WiFiSecurityMethod = "WEP"
	WiFiSecurityWPAWPA2     WiFiSecurityMethod = "WPA/WPA2"
	WiFiSecurityWPA3        WiFiSecurityMethod = "WPA3"
	WiFiSecurityWPA2WPA3    WiFiSecurityMethod = "WPA2/WPA3"
	WiFiSecurityUnsupported WiFiSecurityMethod = "Unsupported"
)

type WiFiIPConfig struct {
	IPMethod WiFiIPMethod `json:"ip_method,omitempty"`
	IPType   WiFiIPType   `json:"ip_type,omitempty"`
	Address  string       `json:"address,omitempty"`
}

type WiFiIPMethod string

const (
	WiFiIPMethodDHCP   WiFiIPMethod = "dhcp"
	WiFiIPMethodStatic WiFiIPMethod = "static"
)

type WiFiIPType string

const (
	WiFiIPTypeIPv4 WiFiIPType = "ipv4"
	WiFiIPTypeIPv6 WiFiIPType = "ipv6"
)

type InputKey string

const (
	InputKeyUp       InputKey = "up"
	InputKeyDown     InputKey = "down"
	InputKeyOK       InputKey = "ok"
	InputKeyBack     InputKey = "back"
	InputKeyStart    InputKey = "start"
	InputKeyBusy     InputKey = "busy"
	InputKeyCustom   InputKey = "custom"
	InputKeyOff      InputKey = "off"
	InputKeyApps     InputKey = "apps"
	InputKeySettings InputKey = "settings"
)

type SmartHomePairingInfo struct {
	FabricCount         int                       `json:"fabric_count"`
	LatestPairingStatus SmartHomePairingStatusRef `json:"latest_pairing_status"`
}

type SmartHomePairingStatusRef struct {
	Value     SmartHomePairingStatus `json:"value"`
	Timestamp int64                  `json:"timestamp,omitempty"`
}

type SmartHomePairingStatus string

const (
	SmartHomePairingNeverStarted          SmartHomePairingStatus = "never_started"
	SmartHomePairingStarted               SmartHomePairingStatus = "started"
	SmartHomePairingCompletedSuccessfully SmartHomePairingStatus = "completed_successfully"
	SmartHomePairingFailed                SmartHomePairingStatus = "failed"
)

type SmartHomePairingPayload struct {
	AvailableUntil string `json:"available_until"`
	QRCode         string `json:"qr_code"`
	ManualCode     string `json:"manual_code"`
}

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

type SmartHomeSwitchStartup string

const (
	SmartHomeSwitchStartupOff    SmartHomeSwitchStartup = "off"
	SmartHomeSwitchStartupOn     SmartHomeSwitchStartup = "on"
	SmartHomeSwitchStartupToggle SmartHomeSwitchStartup = "toggle"
	SmartHomeSwitchStartupLast   SmartHomeSwitchStartup = "last"
)

type TimestampInfo struct {
	Timestamp string `json:"timestamp"`
}

type TimezoneInfo struct {
	Name   string `json:"name"`
	Offset string `json:"offset"`
	Abbr   string `json:"abbr"`
}

type TimezoneListResponse struct {
	List []TimezoneInfo `json:"list"`
}

type UpdateStatus struct {
	Install UpdateInstallStatus `json:"install"`
	Check   UpdateCheckStatus   `json:"check"`
}

type UpdateInstallStatus struct {
	IsAllowed bool                 `json:"is_allowed"`
	Event     UpdateInstallEvent   `json:"event"`
	Action    UpdateInstallAction  `json:"action"`
	Status    UpdateInstallResult  `json:"status"`
	Detail    string               `json:"detail,omitempty"`
	Download  UpdateDownloadStatus `json:"download"`
}

type UpdateDownloadStatus struct {
	SpeedBytesPerSec int64 `json:"speed_bytes_per_sec"`
	ReceivedBytes    int64 `json:"received_bytes"`
	TotalBytes       int64 `json:"total_bytes"`
}

type UpdateCheckStatus struct {
	AvailableVersion string            `json:"available_version"`
	Event            UpdateCheckEvent  `json:"event"`
	Status           UpdateCheckResult `json:"status"`
}

type UpdateInstallEvent string

const (
	UpdateInstallEventSessionStart   UpdateInstallEvent = "session_start"
	UpdateInstallEventSessionStop    UpdateInstallEvent = "session_stop"
	UpdateInstallEventActionBegin    UpdateInstallEvent = "action_begin"
	UpdateInstallEventActionDone     UpdateInstallEvent = "action_done"
	UpdateInstallEventDetailChange   UpdateInstallEvent = "detail_change"
	UpdateInstallEventActionProgress UpdateInstallEvent = "action_progress"
	UpdateInstallEventNone           UpdateInstallEvent = "none"
)

type UpdateInstallAction string

const (
	UpdateInstallActionDownload        UpdateInstallAction = "download"
	UpdateInstallActionSHAVerification UpdateInstallAction = "sha_verification"
	UpdateInstallActionUnpack          UpdateInstallAction = "unpack"
	UpdateInstallActionPrepare         UpdateInstallAction = "prepare"
	UpdateInstallActionApply           UpdateInstallAction = "apply"
	UpdateInstallActionNone            UpdateInstallAction = "none"
)

type UpdateInstallResult string

const (
	UpdateInstallOK                         UpdateInstallResult = "ok"
	UpdateInstallBatteryLow                 UpdateInstallResult = "battery_low"
	UpdateInstallBusy                       UpdateInstallResult = "busy"
	UpdateInstallDownloadFailure            UpdateInstallResult = "download_failure"
	UpdateInstallDownloadAbort              UpdateInstallResult = "download_abort"
	UpdateInstallSHAMismatch                UpdateInstallResult = "sha_mismatch"
	UpdateInstallUnpackStagingDirFailure    UpdateInstallResult = "unpack_staging_dir_failure"
	UpdateInstallUnpackArchiveOpenFailure   UpdateInstallResult = "unpack_archive_open_failure"
	UpdateInstallUnpackArchiveUnpackFailure UpdateInstallResult = "unpack_archive_unpack_failure"
	UpdateInstallManifestNotFound           UpdateInstallResult = "install_manifest_not_found"
	UpdateInstallManifestInvalid            UpdateInstallResult = "install_manifest_invalid"
	UpdateInstallSessionConfigFailure       UpdateInstallResult = "install_session_config_failure"
	UpdateInstallPointerSetupFailure        UpdateInstallResult = "install_pointer_setup_failure"
	UpdateInstallUnknownFailure             UpdateInstallResult = "unknown_failure"
)

type UpdateCheckEvent string

const (
	UpdateCheckEventStart UpdateCheckEvent = "start"
	UpdateCheckEventStop  UpdateCheckEvent = "stop"
	UpdateCheckEventNone  UpdateCheckEvent = "none"
)

type UpdateCheckResult string

const (
	UpdateCheckAvailable    UpdateCheckResult = "available"
	UpdateCheckNotAvailable UpdateCheckResult = "not_available"
	UpdateCheckFailure      UpdateCheckResult = "failure"
	UpdateCheckNone         UpdateCheckResult = "none"
)

type UpdateChangelog struct {
	Changelog string `json:"changelog"`
}

type AutoupdateSettings struct {
	IsEnabled     *bool  `json:"is_enabled,omitempty"`
	IntervalStart string `json:"interval_start,omitempty"`
	IntervalEnd   string `json:"interval_end,omitempty"`
}
