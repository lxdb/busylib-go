package snapshot_test

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	busylib "github.com/lxdb/busylib-go"
	"github.com/lxdb/busylib-go/proto/blepb"
	"github.com/lxdb/busylib-go/proto/framepb"
	"github.com/lxdb/busylib-go/proto/inputpb"
	"github.com/lxdb/busylib-go/proto/statepb"
	"github.com/lxdb/busylib-go/proto/timerpb"
	"github.com/lxdb/busylib-go/proto/updatepb"
	"github.com/lxdb/busylib-go/proto/utilpb"
	"github.com/lxdb/busylib-go/snapshot"
	"github.com/lxdb/busylib-go/stream"
	"google.golang.org/protobuf/proto"
)

func TestStoreApplyRetainsAllFirmwareTypedUpdates(t *testing.T) {
	updates := allFirmwareUpdates()
	store := snapshot.NewStore(snapshot.Snapshot{})

	change := store.Apply(updates...)
	wantSections := []snapshot.Section{
		snapshot.SectionName,
		snapshot.SectionPower,
		snapshot.SectionBrightness,
		snapshot.SectionAudioVolume,
		snapshot.SectionWiFi,
		snapshot.SectionFirmwareUpdate,
		snapshot.SectionUpdateCheck,
		snapshot.SectionTimezone,
		snapshot.SectionMatter,
		snapshot.SectionFrame,
		snapshot.SectionInput,
		snapshot.SectionTimer,
		snapshot.SectionBLE,
		snapshot.SectionAutoUpdate,
		snapshot.SectionTimerProfiles,
	}
	if !reflect.DeepEqual(change.Sections, wantSections) {
		t.Fatalf("sections = %#v, want %#v", change.Sections, wantSections)
	}
	got := change.Snapshot
	if got.Name.Value != "Live Desk" || !got.Power.Value.Known || got.Power.Value.BatteryCurrentMA != -90 {
		t.Fatalf("name=%#v power=%#v", got.Name, got.Power)
	}
	if got.Brightness.Value.Mode != snapshot.BrightnessModeManual || got.Brightness.Value.Manual == nil || *got.Brightness.Value.Manual != 35 || got.Brightness.Value.Actual == nil || *got.Brightness.Value.Actual != 30 {
		t.Fatalf("brightness = %#v", got.Brightness.Value)
	}
	if got.AudioVolume.Value != 55 || got.WiFi.Value.State != snapshot.WiFiStateActive || len(got.WiFi.Value.IPAddresses) != 1 {
		t.Fatalf("audio=%#v wifi=%#v", got.AudioVolume, got.WiFi)
	}
	if got.BLE.Value.Status != blepb.ServiceStatus_CONNECTABLE {
		t.Fatalf("BLE = %#v", got.BLE.Value)
	}
	assertProtoField(t, "firmware", got.FirmwareUpdate.Present, got.FirmwareUpdate.Value, updates[5].Proto())
	assertProtoField(t, "update check", got.UpdateCheck.Present, got.UpdateCheck.Value, updates[6].Proto())
	assertProtoField(t, "timezone", got.Timezone.Present, got.Timezone.Value, updates[7].Proto())
	assertProtoField(t, "matter", got.Matter.Present, got.Matter.Value, updates[8].Proto())
	assertProtoField(t, "frame", got.Frame.Present, got.Frame.Value, updates[9].Proto())
	assertProtoField(t, "input", got.Input.Present, got.Input.Value, updates[10].Proto())
	assertProtoField(t, "timer", got.Timer.Present, got.Timer.Value, updates[11].Proto())
	assertProtoField(t, "auto update", got.AutoUpdate.Present, got.AutoUpdate.Value, updates[13].Proto())
	assertProtoField(t, "timer profiles", got.TimerProfiles.Present, got.TimerProfiles.Value, updates[14].Proto())
}

func TestStoreApplyPreservesMissingSectionsAndReportsFinalChangesOnce(t *testing.T) {
	initialError := errors.New("name unavailable")
	initial := snapshot.Snapshot{
		Name: snapshot.Field[string]{Err: initialError, Raw: []byte("bad-name")},
		Status: snapshot.Field[busylib.Status]{
			Present: true,
			Value:   busylib.Status{System: busylib.SystemStatus{Uptime: "1m"}},
		},
		Time: snapshot.Field[snapshot.DeviceTime]{
			Present: true,
			Value:   snapshot.DeviceTime{Timestamp: "2026-07-15T12:30:45-06:00"},
		},
	}
	store := snapshot.NewStore(initial)

	change := store.Apply(
		stream.DeviceNameUpdate{Value: &statepb.DeviceName{Name: "Temporary"}},
		stream.DeviceNameUpdate{Value: &statepb.DeviceName{Name: "Desk"}},
		stream.PowerUpdate{Value: &statepb.Power{State: &statepb.Power_Unknown{Unknown: &statepb.UnknownPowerState{}}}},
	)
	if !reflect.DeepEqual(change.Sections, []snapshot.Section{snapshot.SectionName, snapshot.SectionPower}) {
		t.Fatalf("sections = %#v", change.Sections)
	}
	if change.Snapshot.Name.Value != "Desk" || change.Snapshot.Name.Err != nil || change.Snapshot.Name.Raw != nil {
		t.Fatalf("name = %#v", change.Snapshot.Name)
	}
	if !change.Snapshot.Status.Present || change.Snapshot.Status.Value.System.Uptime != "1m" || !change.Snapshot.Time.Present {
		t.Fatalf("unrelated fields changed: status=%#v time=%#v", change.Snapshot.Status, change.Snapshot.Time)
	}
	if change.Snapshot.Power.Value.Known {
		t.Fatalf("unknown power = %#v", change.Snapshot.Power.Value)
	}

	unchanged := store.Apply(
		stream.DeviceNameUpdate{Value: &statepb.DeviceName{Name: "Temporary"}},
		stream.DeviceNameUpdate{Value: &statepb.DeviceName{Name: "Desk"}},
		stream.PowerUpdate{Value: &statepb.Power{State: &statepb.Power_Unknown{Unknown: &statepb.UnknownPowerState{}}}},
		nil,
		stream.UnknownUpdate{Value: &statepb.StateUpdate{}},
	)
	if len(unchanged.Sections) != 0 {
		t.Fatalf("unchanged sections = %#v", unchanged.Sections)
	}
}

func TestStoreDefensivelyCopiesInputsAndSnapshots(t *testing.T) {
	frame := &framepb.Frame{Data: []byte{1, 2, 3}}
	timer := &timerpb.Timer{Json: &utilpb.Json{Data: []byte(`{"running":true}`)}}
	wifi := &statepb.Wifi{
		WifiState: &statepb.Wifi_Active{Active: &statepb.WifiStateActive{Ssid: "Desk"}},
		IpAddresses: []*statepb.IpAddress{{
			Protocol: statepb.IpProtocol_IPV4,
			Method:   statepb.IpConfigurationMethod_DHCP,
			Address:  "192.168.1.20",
		}},
	}
	store := snapshot.NewStore(snapshot.Snapshot{})
	store.Apply(
		stream.FrameUpdate{Value: frame},
		stream.TimerUpdate{Value: timer},
		stream.WiFiUpdate{Value: wifi},
	)

	frame.Data[0] = 9
	timer.Json.Data[0] = 'x'
	wifi.IpAddresses[0].Address = "mutated"
	first := store.Snapshot()
	if first.Frame.Value.Data[0] != 1 || first.Timer.Value.Json.Data[0] != '{' || first.WiFi.Value.IPAddresses[0].Address != "192.168.1.20" {
		t.Fatalf("store retained caller mutation: frame=%v timer=%q wifi=%#v", first.Frame.Value.Data, first.Timer.Value.Json.Data, first.WiFi.Value)
	}

	first.Frame.Value.Data[0] = 8
	first.Timer.Value.Json.Data[0] = 'y'
	first.WiFi.Value.IPAddresses[0].Address = "changed-copy"
	second := store.Snapshot()
	if second.Frame.Value.Data[0] != 1 || second.Timer.Value.Json.Data[0] != '{' || second.WiFi.Value.IPAddresses[0].Address != "192.168.1.20" {
		t.Fatalf("Snapshot exposed store state: frame=%v timer=%q wifi=%#v", second.Frame.Value.Data, second.Timer.Value.Json.Data, second.WiFi.Value)
	}
}

func TestStoreSupportsConcurrentApplyAndSnapshot(t *testing.T) {
	store := snapshot.NewStore(snapshot.Snapshot{})
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for iteration := 0; iteration < 100; iteration++ {
				store.Apply(stream.AudioVolumeUpdate{Value: &statepb.AudioVolume{Volume: uint32((worker + iteration) % 101)}})
				_ = store.Snapshot()
			}
		}(worker)
	}
	wg.Wait()
	if !store.Snapshot().AudioVolume.Present {
		t.Fatal("audio volume was never stored")
	}
}

func TestSnapshotWithOnlyStreamStateIsNotEmpty(t *testing.T) {
	value := snapshot.Snapshot{
		Frame: snapshot.Field[*framepb.Frame]{
			Present: true,
			Value:   &framepb.Frame{Screen: framepb.Screen_FRONT},
		},
	}
	if value.Empty() {
		t.Fatal("Snapshot with a stream-only field is empty")
	}
}

func assertProtoField(t *testing.T, name string, present bool, got, want proto.Message) {
	t.Helper()
	if !present || !proto.Equal(got, want) {
		t.Fatalf("%s present=%v got=%v want=%v", name, present, got, want)
	}
}

func allFirmwareUpdates() []stream.Update {
	return []stream.Update{
		stream.DeviceNameUpdate{Value: &statepb.DeviceName{Name: "Live Desk"}},
		stream.PowerUpdate{Value: &statepb.Power{State: &statepb.Power_Known{Known: &statepb.PowerState{
			BatteryStatus:        statepb.BatteryStatus_DISCHARGING,
			BatteryChargePercent: 60,
			BatteryVoltageMv:     3750,
			BatteryCurrentMa:     -90,
			UsbVoltageMv:         5000,
		}}}},
		stream.BrightnessUpdate{Value: &statepb.Brightness{
			Setting:          &statepb.Brightness_Manual{Manual: &statepb.BrightnessManual{Brightness: 35}},
			ActualBrightness: 30,
		}},
		stream.AudioVolumeUpdate{Value: &statepb.AudioVolume{Volume: 55}},
		stream.WiFiUpdate{Value: &statepb.Wifi{
			WifiState: &statepb.Wifi_Active{Active: &statepb.WifiStateActive{
				Status:   statepb.WifiConnectionStatus_CONNECTED,
				Ssid:     "Desk",
				Bssid:    "11:22:33:44:55:66",
				Channel:  6,
				Rssi:     -40,
				Security: statepb.WifiSecurity_WPA2,
			}},
			IpAddresses: []*statepb.IpAddress{{
				Protocol: statepb.IpProtocol_IPV4,
				Method:   statepb.IpConfigurationMethod_DHCP,
				Address:  "192.168.1.20",
				Gateway:  "192.168.1.1",
				Netmask:  "255.255.255.0",
			}},
		}},
		stream.FirmwareUpdate{Value: &updatepb.UpdateState{Event: updatepb.UpdateEvent_ACTION_BEGIN}},
		stream.UpdateCheckUpdate{Value: &updatepb.CheckState{
			Event:  updatepb.CheckEvent_STOP,
			Status: &updatepb.CheckState_Available{Available: &updatepb.UpdateAvailable{Version: "2.0.0"}},
		}},
		stream.TimezoneUpdate{Value: &statepb.Timezone{Name: "America/Monterrey", Offset: -360, Abbr: "CST"}},
		stream.MatterUpdate{Value: &statepb.Matter{FabricCount: 2}},
		stream.FrameUpdate{Value: &framepb.Frame{Screen: framepb.Screen_FRONT, Width: 1, Height: 1, Data: []byte{1, 2, 3}}},
		stream.InputUpdate{Value: &inputpb.InputEvent{Event: &inputpb.InputEvent_ButtonEvent{ButtonEvent: &inputpb.ButtonEvent{Button: inputpb.Button_OK, Action: inputpb.ButtonAction_PRESS}}}},
		stream.TimerUpdate{Value: &timerpb.Timer{Json: &utilpb.Json{Data: []byte(`{"running":true}`)}}},
		stream.BLEUpdate{Value: &blepb.Ble{Status: blepb.ServiceStatus_CONNECTABLE}},
		stream.AutoUpdateUpdate{Value: &updatepb.AutoUpdateState{Enabled: true, Interval: &updatepb.AutoUpdateInterval{Start: 60, End: 120}}},
		stream.TimerProfilesUpdate{Value: &timerpb.Profiles{Profiles: []*timerpb.Profile{{Name: "busy", Json: &utilpb.Json{Data: []byte(`{}`)}}}}},
	}
}
