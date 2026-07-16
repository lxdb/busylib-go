package snapshot

import (
	"bytes"
	"reflect"
	"sync"

	busylib "github.com/lxdb/busylib-go"
	"github.com/lxdb/busylib-go/proto/blepb"
	"github.com/lxdb/busylib-go/proto/framepb"
	"github.com/lxdb/busylib-go/proto/inputpb"
	"github.com/lxdb/busylib-go/proto/statepb"
	"github.com/lxdb/busylib-go/proto/timerpb"
	"github.com/lxdb/busylib-go/proto/updatepb"
	publicstream "github.com/lxdb/busylib-go/stream"
	"google.golang.org/protobuf/proto"
)

// Change is the store state after a batch together with sections whose final
// values differ from their values before the batch.
type Change struct {
	Snapshot Snapshot
	Sections []Section
}

// Store retains the latest snapshot and status-stream values. It does not own
// a stream, background work, or a notification channel.
type Store struct {
	mu       sync.RWMutex
	snapshot Snapshot
}

func NewStore(initial Snapshot) *Store {
	return &Store{snapshot: cloneSnapshot(initial)}
}

func (s *Store) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSnapshot(s.snapshot)
}

// Apply merges typed updates in order, then reports each section whose final
// value changed. Nil and unknown update kinds are ignored.
func (s *Store) Apply(updates ...publicstream.Update) Change {
	if s == nil {
		return Change{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	before := cloneSnapshot(s.snapshot)
	touched := make(map[Section]bool)
	for _, update := range updates {
		s.apply(update, touched)
	}

	sections := changedSections(before, s.snapshot, touched)
	return Change{Snapshot: cloneSnapshot(s.snapshot), Sections: sections}
}

func (s *Store) apply(update publicstream.Update, touched map[Section]bool) {
	switch update := update.(type) {
	case publicstream.DeviceNameUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.Name = present(update.Value.GetName())
		touched[SectionName] = true
	case publicstream.PowerUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.Power = present(powerFromProto(update.Value))
		touched[SectionPower] = true
	case publicstream.BrightnessUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.Brightness = present(brightnessFromProto(update.Value))
		touched[SectionBrightness] = true
	case publicstream.AudioVolumeUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.AudioVolume = present(int(update.Value.GetVolume()))
		touched[SectionAudioVolume] = true
	case publicstream.WiFiUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.WiFi = present(wifiFromProto(update.Value))
		touched[SectionWiFi] = true
	case publicstream.FirmwareUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.FirmwareUpdate = present(proto.Clone(update.Value).(*updatepb.UpdateState))
		touched[SectionFirmwareUpdate] = true
	case publicstream.UpdateCheckUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.UpdateCheck = present(proto.Clone(update.Value).(*updatepb.CheckState))
		touched[SectionUpdateCheck] = true
	case publicstream.TimezoneUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.Timezone = present(proto.Clone(update.Value).(*statepb.Timezone))
		touched[SectionTimezone] = true
	case publicstream.MatterUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.Matter = present(proto.Clone(update.Value).(*statepb.Matter))
		touched[SectionMatter] = true
	case publicstream.FrameUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.Frame = present(proto.Clone(update.Value).(*framepb.Frame))
		touched[SectionFrame] = true
	case publicstream.InputUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.Input = present(proto.Clone(update.Value).(*inputpb.InputEvent))
		touched[SectionInput] = true
	case publicstream.TimerUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.Timer = present(proto.Clone(update.Value).(*timerpb.Timer))
		touched[SectionTimer] = true
	case publicstream.BLEUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.BLE = present(bleFromProto(update.Value))
		touched[SectionBLE] = true
	case publicstream.AutoUpdateUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.AutoUpdate = present(proto.Clone(update.Value).(*updatepb.AutoUpdateState))
		touched[SectionAutoUpdate] = true
	case publicstream.TimerProfilesUpdate:
		if update.Value == nil {
			return
		}
		s.snapshot.TimerProfiles = present(proto.Clone(update.Value).(*timerpb.Profiles))
		touched[SectionTimerProfiles] = true
	}
}

var streamSectionOrder = []Section{
	SectionName,
	SectionPower,
	SectionBrightness,
	SectionAudioVolume,
	SectionWiFi,
	SectionFirmwareUpdate,
	SectionUpdateCheck,
	SectionTimezone,
	SectionMatter,
	SectionFrame,
	SectionInput,
	SectionTimer,
	SectionBLE,
	SectionAutoUpdate,
	SectionTimerProfiles,
}

func changedSections(before, after Snapshot, touched map[Section]bool) []Section {
	var changed []Section
	for _, section := range streamSectionOrder {
		if touched[section] && !sectionEqual(section, before, after) {
			changed = append(changed, section)
		}
	}
	return changed
}

func sectionEqual(section Section, before, after Snapshot) bool {
	switch section {
	case SectionName:
		return reflect.DeepEqual(before.Name, after.Name)
	case SectionPower:
		return reflect.DeepEqual(before.Power, after.Power)
	case SectionBrightness:
		return reflect.DeepEqual(before.Brightness, after.Brightness)
	case SectionAudioVolume:
		return reflect.DeepEqual(before.AudioVolume, after.AudioVolume)
	case SectionWiFi:
		return reflect.DeepEqual(before.WiFi, after.WiFi)
	case SectionFirmwareUpdate:
		return protoFieldEqual(before.FirmwareUpdate, after.FirmwareUpdate)
	case SectionUpdateCheck:
		return protoFieldEqual(before.UpdateCheck, after.UpdateCheck)
	case SectionTimezone:
		return protoFieldEqual(before.Timezone, after.Timezone)
	case SectionMatter:
		return protoFieldEqual(before.Matter, after.Matter)
	case SectionFrame:
		return protoFieldEqual(before.Frame, after.Frame)
	case SectionInput:
		return protoFieldEqual(before.Input, after.Input)
	case SectionTimer:
		return protoFieldEqual(before.Timer, after.Timer)
	case SectionBLE:
		return reflect.DeepEqual(before.BLE, after.BLE)
	case SectionAutoUpdate:
		return protoFieldEqual(before.AutoUpdate, after.AutoUpdate)
	case SectionTimerProfiles:
		return protoFieldEqual(before.TimerProfiles, after.TimerProfiles)
	default:
		return true
	}
}

func powerFromProto(value *statepb.Power) Power {
	known := value.GetKnown()
	if known == nil {
		return Power{}
	}
	return Power{
		Known:                true,
		BatteryStatus:        known.GetBatteryStatus(),
		BatteryChargePercent: known.GetBatteryChargePercent(),
		BatteryVoltageMV:     known.GetBatteryVoltageMv(),
		BatteryCurrentMA:     known.GetBatteryCurrentMa(),
		USBVoltageMV:         known.GetUsbVoltageMv(),
	}
}

func brightnessFromProto(value *statepb.Brightness) Brightness {
	result := Brightness{Mode: BrightnessModeUnknown}
	if value.GetAutomatic() != nil {
		result.Mode = BrightnessModeAutomatic
	} else if manual := value.GetManual(); manual != nil {
		result.Mode = BrightnessModeManual
		setting := manual.GetBrightness()
		result.Manual = &setting
	}
	actual := value.GetActualBrightness()
	result.Actual = &actual
	return result
}

func wifiFromProto(value *statepb.Wifi) WiFi {
	result := WiFi{State: WiFiStateUnknown}
	if value.GetInactive() != nil {
		result.State = WiFiStateInactive
	} else if active := value.GetActive(); active != nil {
		result.State = WiFiStateActive
		status := active.GetStatus()
		result.ConnectionStatus = &status
		result.SSID = active.GetSsid()
		result.BSSID = active.GetBssid()
		result.Channel = active.GetChannel()
		result.RSSI = active.GetRssi()
		security := active.GetSecurity()
		result.Security = wifiSecurityName(security)
		result.SecurityCode = &security
	}
	result.IPAddresses = make([]IPAddress, 0, len(value.GetIpAddresses()))
	for _, address := range value.GetIpAddresses() {
		if address == nil {
			continue
		}
		result.IPAddresses = append(result.IPAddresses, IPAddress{
			Protocol: address.GetProtocol(),
			Method:   address.GetMethod(),
			Address:  address.GetAddress(),
			Gateway:  address.GetGateway(),
			Netmask:  address.GetNetmask(),
		})
	}
	return result
}

func wifiSecurityName(value statepb.WifiSecurity) busylib.WiFiSecurityMethod {
	switch value {
	case statepb.WifiSecurity_OPEN:
		return busylib.WiFiSecurityOpen
	case statepb.WifiSecurity_WPA:
		return busylib.WiFiSecurityWPA
	case statepb.WifiSecurity_WPA2:
		return busylib.WiFiSecurityWPA2
	case statepb.WifiSecurity_WEP:
		return busylib.WiFiSecurityWEP
	case statepb.WifiSecurity_WPA_WPA2:
		return busylib.WiFiSecurityWPAWPA2
	case statepb.WifiSecurity_WPA3:
		return busylib.WiFiSecurityWPA3
	case statepb.WifiSecurity_WPA2_WPA3:
		return busylib.WiFiSecurityWPA2WPA3
	default:
		return busylib.WiFiSecurityUnsupported
	}
}

func bleFromProto(value *blepb.Ble) BLE {
	result := BLE{Status: value.GetStatus()}
	if value.RemoteAddress != nil {
		address := value.GetRemoteAddress()
		result.Address = &address
	}
	return result
}

func cloneSnapshot(value Snapshot) Snapshot {
	result := value
	result.Name = cloneField(value.Name)
	result.Version = cloneField(value.Version)
	result.Status = cloneField(value.Status)
	result.System = cloneField(value.System)
	result.Power = cloneField(value.Power)
	result.Time = cloneField(value.Time)
	result.WiFi = cloneField(value.WiFi)
	result.Brightness = cloneField(value.Brightness)
	result.AudioVolume = cloneField(value.AudioVolume)
	result.BLE = cloneField(value.BLE)
	result.Storage = cloneField(value.Storage)

	result.Brightness.Value = cloneBrightness(value.Brightness.Value)
	result.WiFi.Value = cloneWiFi(value.WiFi.Value)
	result.BLE.Value = cloneBLE(value.BLE.Value)
	result.FirmwareUpdate = cloneProtoField(value.FirmwareUpdate)
	result.UpdateCheck = cloneProtoField(value.UpdateCheck)
	result.Timezone = cloneProtoField(value.Timezone)
	result.Matter = cloneProtoField(value.Matter)
	result.Frame = cloneProtoField(value.Frame)
	result.Input = cloneProtoField(value.Input)
	result.Timer = cloneProtoField(value.Timer)
	result.AutoUpdate = cloneProtoField(value.AutoUpdate)
	result.TimerProfiles = cloneProtoField(value.TimerProfiles)
	return result
}

func cloneField[T any](value Field[T]) Field[T] {
	value.Raw = append([]byte(nil), value.Raw...)
	return value
}

func cloneBrightness(value Brightness) Brightness {
	if value.Manual != nil {
		manual := *value.Manual
		value.Manual = &manual
	}
	if value.Actual != nil {
		actual := *value.Actual
		value.Actual = &actual
	}
	return value
}

func cloneWiFi(value WiFi) WiFi {
	if value.ConnectionStatus != nil {
		status := *value.ConnectionStatus
		value.ConnectionStatus = &status
	}
	if value.SecurityCode != nil {
		security := *value.SecurityCode
		value.SecurityCode = &security
	}
	value.IPAddresses = append([]IPAddress(nil), value.IPAddresses...)
	return value
}

func cloneBLE(value BLE) BLE {
	if value.Address != nil {
		address := *value.Address
		value.Address = &address
	}
	return value
}

func cloneProtoField[T proto.Message](value Field[T]) Field[T] {
	result := cloneField(value)
	if !nilProto(value.Value) {
		result.Value = proto.Clone(value.Value).(T)
	}
	return result
}

func protoFieldEqual[T proto.Message](left, right Field[T]) bool {
	return left.Present == right.Present &&
		reflect.DeepEqual(left.Err, right.Err) &&
		bytes.Equal(left.Raw, right.Raw) &&
		proto.Equal(left.Value, right.Value)
}

func nilProto(value proto.Message) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	return reflected.Kind() == reflect.Pointer && reflected.IsNil()
}
