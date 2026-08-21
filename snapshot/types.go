package snapshot

import (
	"time"

	busylib "github.com/lxdb/busylib-go"
	"github.com/lxdb/busylib-go/proto/blepb"
	"github.com/lxdb/busylib-go/proto/framepb"
	"github.com/lxdb/busylib-go/proto/inputpb"
	"github.com/lxdb/busylib-go/proto/statepb"
	"github.com/lxdb/busylib-go/proto/timerpb"
	"github.com/lxdb/busylib-go/proto/updatepb"
)

// Field retains one snapshot section, its availability, and any field-local
// failure. Raw is populated when a successful HTTP response cannot be decoded.
type Field[T any] struct {
	Value   T
	Present bool
	Err     error
	Raw     []byte
}

// Section identifies one independently collected or updated snapshot field.
type Section string

const (
	// SectionName identifies the device name field.
	SectionName Section = "name"
	// SectionVersion identifies the API version field.
	SectionVersion Section = "version"
	// SectionStatus identifies the aggregate status field.
	SectionStatus Section = "status"
	// SectionSystem identifies the system status field.
	SectionSystem Section = "system"
	// SectionPower identifies the power status field.
	SectionPower Section = "power"
	// SectionTime identifies the device time field.
	SectionTime Section = "time"
	// SectionWiFi identifies the Wi-Fi status field.
	SectionWiFi Section = "wifi"
	// SectionBrightness identifies the display brightness field.
	SectionBrightness Section = "brightness"
	// SectionAudioVolume identifies the audio volume field.
	SectionAudioVolume Section = "audio_volume"
	// SectionBLE identifies the Bluetooth Low Energy status field.
	SectionBLE Section = "ble"
	// SectionStorage identifies the storage status field.
	SectionStorage Section = "storage"
	// SectionFirmwareUpdate identifies the firmware update field.
	SectionFirmwareUpdate Section = "firmware_update"
	// SectionUpdateCheck identifies the update check field.
	SectionUpdateCheck Section = "update_check"
	// SectionTimezone identifies the timezone field.
	SectionTimezone Section = "timezone"
	// SectionMatter identifies the Matter status field.
	SectionMatter Section = "matter"
	// SectionFrame identifies the display frame field.
	SectionFrame Section = "frame"
	// SectionInput identifies the input event field.
	SectionInput Section = "input"
	// SectionTimer identifies the timer field.
	SectionTimer Section = "timer"
	// SectionAutoUpdate identifies the automatic update field.
	SectionAutoUpdate Section = "auto_update"
	// SectionTimerProfiles identifies the timer profiles field.
	SectionTimerProfiles Section = "timer_profiles"
)

// DeviceTime retains the firmware timestamp and its parsed value.
// Time is zero when Timestamp cannot be parsed.
type DeviceTime struct {
	Timestamp string
	Time      time.Time
}

// Power is a normalized device power snapshot.
// Known is false when the firmware reports an unknown power state.
type Power struct {
	Known                bool
	BatteryStatus        statepb.BatteryStatus
	BatteryChargePercent uint32
	BatteryVoltageMV     uint32
	BatteryCurrentMA     int32
	USBVoltageMV         uint32
}

// BrightnessMode identifies how the firmware controls display brightness.
type BrightnessMode string

const (
	// BrightnessModeUnknown means the firmware value was not recognized.
	BrightnessModeUnknown BrightnessMode = "unknown"
	// BrightnessModeAutomatic means the device selects its brightness.
	BrightnessModeAutomatic BrightnessMode = "automatic"
	// BrightnessModeManual means Manual contains the configured brightness.
	BrightnessModeManual BrightnessMode = "manual"
)

// Brightness retains the configured mode and available brightness values.
// Manual and Actual are nil when the firmware omits those values.
type Brightness struct {
	Mode   BrightnessMode
	Manual *uint32
	Actual *uint32
}

// WiFiState identifies the normalized state of the Wi-Fi service.
type WiFiState string

const (
	// WiFiStateUnknown means the firmware state was not recognized.
	WiFiStateUnknown WiFiState = "unknown"
	// WiFiStateInactive means the Wi-Fi service is not active.
	WiFiStateInactive WiFiState = "inactive"
	// WiFiStateActive means the Wi-Fi service is active.
	WiFiStateActive WiFiState = "active"
)

// IPAddress retains one firmware network configuration entry.
type IPAddress struct {
	Protocol statepb.IpProtocol
	Method   statepb.IpConfigurationMethod
	Address  string
	Gateway  string
	Netmask  string
}

// WiFi is a normalized Wi-Fi status snapshot.
// Optional firmware enum values remain nil when the response omits them.
type WiFi struct {
	State            WiFiState
	ConnectionStatus *statepb.WifiConnectionStatus
	SSID             string
	BSSID            string
	Channel          uint32
	RSSI             int32
	Security         busylib.WiFiSecurityMethod
	SecurityCode     *statepb.WifiSecurity
	IPAddresses      []IPAddress
}

// BLE retains the Bluetooth Low Energy service status and optional address.
type BLE struct {
	Status  blepb.ServiceStatus
	Address *string
}

// Snapshot is a best-effort point-in-time view. The aggregate Status remains
// the original HTTP response; dedicated fields are independently refreshable.
type Snapshot struct {
	Name        Field[string]
	Version     Field[string]
	Status      Field[busylib.Status]
	System      Field[busylib.SystemStatus]
	Power       Field[Power]
	Time        Field[DeviceTime]
	WiFi        Field[WiFi]
	Brightness  Field[Brightness]
	AudioVolume Field[int]
	BLE         Field[BLE]
	Storage     Field[busylib.StorageStatus]

	FirmwareUpdate Field[*updatepb.UpdateState]
	UpdateCheck    Field[*updatepb.CheckState]
	Timezone       Field[*statepb.Timezone]
	Matter         Field[*statepb.Matter]
	Frame          Field[*framepb.Frame]
	Input          Field[*inputpb.InputEvent]
	Timer          Field[*timerpb.Timer]
	AutoUpdate     Field[*updatepb.AutoUpdateState]
	TimerProfiles  Field[*timerpb.Profiles]
}

// Complete reports whether all fields from the HTTP snapshot set are present.
// Status-stream-only fields do not affect the result.
func (s Snapshot) Complete() bool {
	return s.Name.Present && s.Version.Present && s.Status.Present &&
		s.System.Present && s.Power.Present && s.Time.Present &&
		s.WiFi.Present && s.Brightness.Present && s.AudioVolume.Present &&
		s.BLE.Present && s.Storage.Present
}

// Empty reports whether no snapshot field is present.
func (s Snapshot) Empty() bool {
	return !s.Name.Present && !s.Version.Present && !s.Status.Present &&
		!s.System.Present && !s.Power.Present && !s.Time.Present &&
		!s.WiFi.Present && !s.Brightness.Present && !s.AudioVolume.Present &&
		!s.BLE.Present && !s.Storage.Present && !s.FirmwareUpdate.Present &&
		!s.UpdateCheck.Present && !s.Timezone.Present && !s.Matter.Present &&
		!s.Frame.Present && !s.Input.Present && !s.Timer.Present &&
		!s.AutoUpdate.Present && !s.TimerProfiles.Present
}

// Failures returns each field-local collection or decode error.
// The returned map is independent and excludes successful fields.
func (s Snapshot) Failures() map[Section]error {
	failures := make(map[Section]error)
	addFailure(failures, SectionName, s.Name.Err)
	addFailure(failures, SectionVersion, s.Version.Err)
	addFailure(failures, SectionStatus, s.Status.Err)
	addFailure(failures, SectionSystem, s.System.Err)
	addFailure(failures, SectionPower, s.Power.Err)
	addFailure(failures, SectionTime, s.Time.Err)
	addFailure(failures, SectionWiFi, s.WiFi.Err)
	addFailure(failures, SectionBrightness, s.Brightness.Err)
	addFailure(failures, SectionAudioVolume, s.AudioVolume.Err)
	addFailure(failures, SectionBLE, s.BLE.Err)
	addFailure(failures, SectionStorage, s.Storage.Err)
	addFailure(failures, SectionFirmwareUpdate, s.FirmwareUpdate.Err)
	addFailure(failures, SectionUpdateCheck, s.UpdateCheck.Err)
	addFailure(failures, SectionTimezone, s.Timezone.Err)
	addFailure(failures, SectionMatter, s.Matter.Err)
	addFailure(failures, SectionFrame, s.Frame.Err)
	addFailure(failures, SectionInput, s.Input.Err)
	addFailure(failures, SectionTimer, s.Timer.Err)
	addFailure(failures, SectionAutoUpdate, s.AutoUpdate.Err)
	addFailure(failures, SectionTimerProfiles, s.TimerProfiles.Err)
	return failures
}

func addFailure(failures map[Section]error, section Section, err error) {
	if err != nil {
		failures[section] = err
	}
}
