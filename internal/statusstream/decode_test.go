package statusstream

import (
	"reflect"
	"testing"

	"github.com/coder/websocket"
	"github.com/lxdb/busylib-go/proto/blepb"
	"github.com/lxdb/busylib-go/proto/errorpb"
	"github.com/lxdb/busylib-go/proto/framepb"
	"github.com/lxdb/busylib-go/proto/inputpb"
	"github.com/lxdb/busylib-go/proto/statepb"
	"github.com/lxdb/busylib-go/proto/timerpb"
	"github.com/lxdb/busylib-go/proto/updatepb"
	publicstream "github.com/lxdb/busylib-go/stream"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func TestDecodeMessageMapsEveryFirmwareUpdateInOrder(t *testing.T) {
	state := &statepb.State{
		Timestamp: 1234,
		Updates: []*statepb.StateUpdate{
			{State: &statepb.StateUpdate_DeviceName{DeviceName: &statepb.DeviceName{Name: "BUSY Bar"}}},
			{State: &statepb.StateUpdate_Power{Power: &statepb.Power{}}},
			{State: &statepb.StateUpdate_Brightness{Brightness: &statepb.Brightness{}}},
			{State: &statepb.StateUpdate_AudioVolume{AudioVolume: &statepb.AudioVolume{Volume: 40}}},
			{State: &statepb.StateUpdate_Wifi{Wifi: &statepb.Wifi{}}},
			{State: &statepb.StateUpdate_UpdateState{UpdateState: &updatepb.UpdateState{}}},
			{State: &statepb.StateUpdate_UpdateCheck{UpdateCheck: &updatepb.CheckState{}}},
			{State: &statepb.StateUpdate_Timezone{Timezone: &statepb.Timezone{Name: "UTC"}}},
			{State: &statepb.StateUpdate_Matter{Matter: &statepb.Matter{}}},
			{State: &statepb.StateUpdate_Frame{Frame: &framepb.Frame{}}},
			{State: &statepb.StateUpdate_Input{Input: &inputpb.InputEvent{}}},
			{State: &statepb.StateUpdate_Timer{Timer: &timerpb.Timer{}}},
			{State: &statepb.StateUpdate_Ble{Ble: &blepb.Ble{}}},
			{State: &statepb.StateUpdate_AutoUpdateState{AutoUpdateState: &updatepb.AutoUpdateState{}}},
			{State: &statepb.StateUpdate_TimerProfiles{TimerProfiles: &timerpb.Profiles{}}},
		},
	}
	payload, err := proto.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	message, fatal := decodeMessage(websocket.MessageBinary, payload)
	if fatal {
		t.Fatal("ordinary state was classified as fatal")
	}
	if message.DecodeError != nil {
		t.Fatalf("DecodeError = %v", message.DecodeError)
	}
	if !reflect.DeepEqual(message.Raw, payload) {
		t.Fatal("raw payload was not preserved")
	}
	if message.State.GetTimestamp() != 1234 {
		t.Fatalf("timestamp = %d", message.State.GetTimestamp())
	}

	wantKinds := []publicstream.UpdateKind{
		publicstream.UpdateDeviceName,
		publicstream.UpdatePower,
		publicstream.UpdateBrightness,
		publicstream.UpdateAudioVolume,
		publicstream.UpdateWiFi,
		publicstream.UpdateFirmware,
		publicstream.UpdateCheck,
		publicstream.UpdateTimezone,
		publicstream.UpdateMatter,
		publicstream.UpdateFrame,
		publicstream.UpdateInput,
		publicstream.UpdateTimer,
		publicstream.UpdateBLE,
		publicstream.UpdateAutoUpdate,
		publicstream.UpdateTimerProfiles,
	}
	gotKinds := make([]publicstream.UpdateKind, 0, len(message.Updates))
	for _, update := range message.Updates {
		gotKinds = append(gotKinds, update.Kind())
		if update.Proto() == nil {
			t.Fatalf("%s update has nil protobuf payload", update.Kind())
		}
	}
	if !reflect.DeepEqual(gotKinds, wantKinds) {
		t.Fatalf("update kinds = %#v, want %#v", gotKinds, wantKinds)
	}
}

func TestDecodeMessagePreservesTextMalformedAndUnknownUpdates(t *testing.T) {
	textMessage, fatal := decodeMessage(websocket.MessageText, []byte("ready"))
	if fatal || textMessage.Kind != publicstream.MessageText || textMessage.Text != "ready" {
		t.Fatalf("text message = %#v, fatal=%v", textMessage, fatal)
	}

	malformed := []byte{0xff}
	malformedMessage, fatal := decodeMessage(websocket.MessageBinary, malformed)
	if fatal || malformedMessage.DecodeError == nil || malformedMessage.State != nil {
		t.Fatalf("malformed message = %#v, fatal=%v", malformedMessage, fatal)
	}
	if !reflect.DeepEqual(malformedMessage.Raw, malformed) {
		t.Fatal("malformed raw payload was not preserved")
	}

	unknown := new(statepb.StateUpdate)
	unknown.ProtoReflect().SetUnknown(protowire.AppendVarint(
		protowire.AppendTag(nil, 99, protowire.VarintType),
		1,
	))
	payload, err := proto.Marshal(&statepb.State{Updates: []*statepb.StateUpdate{unknown}})
	if err != nil {
		t.Fatalf("marshal unknown update: %v", err)
	}
	unknownMessage, fatal := decodeMessage(websocket.MessageBinary, payload)
	if fatal || len(unknownMessage.Updates) != 1 {
		t.Fatalf("unknown message updates = %d, fatal=%v", len(unknownMessage.Updates), fatal)
	}
	update, ok := unknownMessage.Updates[0].(publicstream.UnknownUpdate)
	if !ok || len(update.Value.ProtoReflect().GetUnknown()) == 0 {
		t.Fatalf("unknown update = %#v", unknownMessage.Updates[0])
	}
}

func TestDecodeMessageDeviceErrorSeverity(t *testing.T) {
	for _, test := range []struct {
		name     string
		severity errorpb.Severity
		fatal    bool
	}{
		{name: "fatal", severity: errorpb.Severity_FATAL, fatal: true},
		{name: "error", severity: errorpb.Severity_ERROR},
		{name: "warning", severity: errorpb.Severity_WARNING},
		{name: "unknown", severity: errorpb.Severity(99)},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload, err := proto.Marshal(&statepb.State{Error: &errorpb.Error{
				Cause:    errorpb.Cause_RESOURCE_LIMIT,
				Severity: test.severity,
			}})
			if err != nil {
				t.Fatalf("marshal device error: %v", err)
			}
			message, fatal := decodeMessage(websocket.MessageBinary, payload)
			if fatal != test.fatal {
				t.Fatalf("fatal = %v, want %v", fatal, test.fatal)
			}
			if message.DeviceError == nil || message.DeviceError.Severity != test.severity {
				t.Fatalf("device error = %#v", message.DeviceError)
			}
		})
	}
}
