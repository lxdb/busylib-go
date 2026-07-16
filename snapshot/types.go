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

type Section string

const (
	SectionName           Section = "name"
	SectionVersion        Section = "version"
	SectionStatus         Section = "status"
	SectionSystem         Section = "system"
	SectionPower          Section = "power"
	SectionTime           Section = "time"
	SectionWiFi           Section = "wifi"
	SectionBrightness     Section = "brightness"
	SectionAudioVolume    Section = "audio_volume"
	SectionBLE            Section = "ble"
	SectionStorage        Section = "storage"
	SectionFirmwareUpdate Section = "firmware_update"
	SectionUpdateCheck    Section = "update_check"
	SectionTimezone       Section = "timezone"
	SectionMatter         Section = "matter"
	SectionFrame          Section = "frame"
	SectionInput          Section = "input"
	SectionTimer          Section = "timer"
	SectionAutoUpdate     Section = "auto_update"
	SectionTimerProfiles  Section = "timer_profiles"
)

type DeviceTime struct {
	Timestamp string
	Time      time.Time
}

type Power struct {
	Known                bool
	BatteryStatus        statepb.BatteryStatus
	BatteryChargePercent uint32
	BatteryVoltageMV     uint32
	BatteryCurrentMA     int32
	USBVoltageMV         uint32
}

type BrightnessMode string

const (
	BrightnessModeUnknown   BrightnessMode = "unknown"
	BrightnessModeAutomatic BrightnessMode = "automatic"
	BrightnessModeManual    BrightnessMode = "manual"
)

type Brightness struct {
	Mode   BrightnessMode
	Manual *uint32
	Actual *uint32
}

type WiFiState string

const (
	WiFiStateUnknown  WiFiState = "unknown"
	WiFiStateInactive WiFiState = "inactive"
	WiFiStateActive   WiFiState = "active"
)

type IPAddress struct {
	Protocol statepb.IpProtocol
	Method   statepb.IpConfigurationMethod
	Address  string
	Gateway  string
	Netmask  string
}

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
	System      Field[busylib.StatusSystem]
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

func (s Snapshot) Complete() bool {
	return s.Name.Present && s.Version.Present && s.Status.Present &&
		s.System.Present && s.Power.Present && s.Time.Present &&
		s.WiFi.Present && s.Brightness.Present && s.AudioVolume.Present &&
		s.BLE.Present && s.Storage.Present
}

func (s Snapshot) Empty() bool {
	return !s.Name.Present && !s.Version.Present && !s.Status.Present &&
		!s.System.Present && !s.Power.Present && !s.Time.Present &&
		!s.WiFi.Present && !s.Brightness.Present && !s.AudioVolume.Present &&
		!s.BLE.Present && !s.Storage.Present && !s.FirmwareUpdate.Present &&
		!s.UpdateCheck.Present && !s.Timezone.Present && !s.Matter.Present &&
		!s.Frame.Present && !s.Input.Present && !s.Timer.Present &&
		!s.AutoUpdate.Present && !s.TimerProfiles.Present
}

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
