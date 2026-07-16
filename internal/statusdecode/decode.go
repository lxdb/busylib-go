package statusdecode

import (
	"time"

	"github.com/lxdb/busylib-go/proto/errorpb"
	"github.com/lxdb/busylib-go/proto/statepb"
	publicstream "github.com/lxdb/busylib-go/stream"
	"google.golang.org/protobuf/proto"
)

// DecodeBinary decodes one firmware BSB_State.State payload for either local
// WebSocket or remote MQTT delivery.
func DecodeBinary(payload []byte, path string) (publicstream.Message, bool) {
	message := publicstream.Message{
		Kind:       publicstream.MessageBinary,
		ReceivedAt: time.Now(),
		Raw:        append([]byte(nil), payload...),
	}
	state := new(statepb.State)
	if err := proto.Unmarshal(payload, state); err != nil {
		message.DecodeError = &publicstream.Error{
			Operation: "decode",
			Path:      path,
			Err:       err,
		}
		return message, false
	}
	message.State = state
	message.Updates = make([]publicstream.Update, 0, len(state.Updates))
	for _, update := range state.Updates {
		message.Updates = append(message.Updates, mapUpdate(update))
	}
	if state.Error != nil {
		message.DeviceError = &publicstream.DeviceError{
			Cause:    state.Error.Cause,
			Severity: state.Error.Severity,
			Raw:      state.Error,
		}
	}
	return message, message.DeviceError != nil && message.DeviceError.Severity == errorpb.Severity_FATAL
}

func mapUpdate(update *statepb.StateUpdate) publicstream.Update {
	if update == nil {
		return publicstream.UnknownUpdate{}
	}
	switch value := update.State.(type) {
	case *statepb.StateUpdate_DeviceName:
		return publicstream.DeviceNameUpdate{Value: value.DeviceName}
	case *statepb.StateUpdate_Power:
		return publicstream.PowerUpdate{Value: value.Power}
	case *statepb.StateUpdate_Brightness:
		return publicstream.BrightnessUpdate{Value: value.Brightness}
	case *statepb.StateUpdate_AudioVolume:
		return publicstream.AudioVolumeUpdate{Value: value.AudioVolume}
	case *statepb.StateUpdate_Wifi:
		return publicstream.WiFiUpdate{Value: value.Wifi}
	case *statepb.StateUpdate_UpdateState:
		return publicstream.FirmwareUpdate{Value: value.UpdateState}
	case *statepb.StateUpdate_UpdateCheck:
		return publicstream.UpdateCheckUpdate{Value: value.UpdateCheck}
	case *statepb.StateUpdate_Timezone:
		return publicstream.TimezoneUpdate{Value: value.Timezone}
	case *statepb.StateUpdate_Matter:
		return publicstream.MatterUpdate{Value: value.Matter}
	case *statepb.StateUpdate_Frame:
		return publicstream.FrameUpdate{Value: value.Frame}
	case *statepb.StateUpdate_Input:
		return publicstream.InputUpdate{Value: value.Input}
	case *statepb.StateUpdate_Timer:
		return publicstream.TimerUpdate{Value: value.Timer}
	case *statepb.StateUpdate_Ble:
		return publicstream.BLEUpdate{Value: value.Ble}
	case *statepb.StateUpdate_AutoUpdateState:
		return publicstream.AutoUpdateUpdate{Value: value.AutoUpdateState}
	case *statepb.StateUpdate_TimerProfiles:
		return publicstream.TimerProfilesUpdate{Value: value.TimerProfiles}
	default:
		return publicstream.UnknownUpdate{Value: update}
	}
}
