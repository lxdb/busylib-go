package stream

import (
	"context"
	"time"

	"github.com/lxdb/busylib-go/proto/blepb"
	"github.com/lxdb/busylib-go/proto/errorpb"
	"github.com/lxdb/busylib-go/proto/framepb"
	"github.com/lxdb/busylib-go/proto/inputpb"
	"github.com/lxdb/busylib-go/proto/statepb"
	"github.com/lxdb/busylib-go/proto/timerpb"
	"github.com/lxdb/busylib-go/proto/updatepb"
	"google.golang.org/protobuf/proto"
)

// Stream is a one-shot BUSY Bar status-stream subscription. Local WebSocket
// and remote MQTT implementations share this lifecycle contract.
type Stream interface {
	Start(context.Context) error
	Stop() error
	RequestSnapshot(context.Context) error
	Messages() <-chan Message
	Statuses() <-chan Status
	Errors() <-chan error
	Status() Status
}

// MessageKind identifies the wire representation of a received message.
type MessageKind string

const (
	// MessageBinary identifies a binary WebSocket or MQTT payload.
	MessageBinary MessageKind = "binary"
	// MessageText identifies a text WebSocket payload.
	MessageText MessageKind = "text"
)

// Message preserves one transport message together with its decoded state and
// ordered product updates. Raw is retained for protocol diagnostics. Text is
// used by the local WebSocket transport; remote MQTT messages are binary.
type Message struct {
	Kind        MessageKind
	ReceivedAt  time.Time
	Raw         []byte
	Text        string
	State       *statepb.State
	Updates     []Update
	DeviceError *DeviceError
	DecodeError error
}

// Lifecycle identifies the connection phase of a status stream.
type Lifecycle string

const (
	// LifecycleIdle means Start has not begun a connection.
	LifecycleIdle Lifecycle = "idle"
	// LifecycleConnecting means the stream is opening its first connection.
	LifecycleConnecting Lifecycle = "connecting"
	// LifecycleConnected means the stream can receive device messages.
	LifecycleConnected Lifecycle = "connected"
	// LifecycleReconnecting means a recoverable failure triggered another attempt.
	LifecycleReconnecting Lifecycle = "reconnecting"
	// LifecycleStopped means the stream ended after Stop or context cancellation.
	LifecycleStopped Lifecycle = "stopped"
	// LifecycleFailed means a terminal error ended the stream.
	LifecycleFailed Lifecycle = "failed"
)

// AccessStatus identifies whether a remote stream lease was accepted.
type AccessStatus string

const (
	// AccessUnknown means no remote access decision has arrived.
	AccessUnknown AccessStatus = "unknown"
	// AccessAccepted means the remote stream lease was accepted.
	AccessAccepted AccessStatus = "accepted"
	// AccessRejected means the remote stream lease was rejected.
	AccessRejected AccessStatus = "rejected"
)

// DataStatus identifies whether the stream has recent device state.
type DataStatus string

const (
	// DataWaiting means no device state has arrived.
	DataWaiting DataStatus = "waiting"
	// DataFresh means the stream received device state within its freshness window.
	DataFresh DataStatus = "fresh"
	// DataStale means the last device state is older than its freshness window.
	DataStale DataStatus = "stale"
)

// Status is the current stream lifecycle snapshot. Statuses may coalesce
// intermediate values; Status always returns the latest value.
type Status struct {
	Lifecycle   Lifecycle
	Access      AccessStatus
	Data        DataStatus
	Attempt     int
	ConnectedAt time.Time
	LastStateAt time.Time
	LastError   error
}

// UpdateKind identifies the protobuf payload carried by an Update.
type UpdateKind string

const (
	// UpdateDeviceName carries a device-name change.
	UpdateDeviceName UpdateKind = "device_name"
	// UpdatePower carries a power-state change.
	UpdatePower UpdateKind = "power"
	// UpdateBrightness carries a display-brightness change.
	UpdateBrightness UpdateKind = "brightness"
	// UpdateAudioVolume carries an audio-volume change.
	UpdateAudioVolume UpdateKind = "audio_volume"
	// UpdateWiFi carries a Wi-Fi state change.
	UpdateWiFi UpdateKind = "wifi"
	// UpdateFirmware carries a firmware-update state change.
	UpdateFirmware UpdateKind = "update_state"
	// UpdateCheck carries a firmware-update check state change.
	UpdateCheck UpdateKind = "update_check"
	// UpdateTimezone carries a timezone change.
	UpdateTimezone UpdateKind = "timezone"
	// UpdateMatter carries a Matter state change.
	UpdateMatter UpdateKind = "matter"
	// UpdateFrame carries a display-frame change.
	UpdateFrame UpdateKind = "frame"
	// UpdateInput carries a device input event.
	UpdateInput UpdateKind = "input"
	// UpdateTimer carries a timer state change.
	UpdateTimer UpdateKind = "timer"
	// UpdateBLE carries a Bluetooth Low Energy state change.
	UpdateBLE UpdateKind = "ble"
	// UpdateAutoUpdate carries an automatic-update state change.
	UpdateAutoUpdate UpdateKind = "auto_update_state"
	// UpdateTimerProfiles carries a timer-profile change.
	UpdateTimerProfiles UpdateKind = "timer_profiles"
	// UpdateUnknown preserves an unrecognized firmware state update.
	UpdateUnknown UpdateKind = "unknown"
)

// Update classifies one firmware StateUpdate while retaining its generated
// protobuf payload and unknown enum values.
type Update interface {
	Kind() UpdateKind
	Proto() proto.Message
}

// DeviceNameUpdate retains a device-name update payload.
type DeviceNameUpdate struct{ Value *statepb.DeviceName }

// PowerUpdate retains a power-state update payload.
type PowerUpdate struct{ Value *statepb.Power }

// BrightnessUpdate retains a display-brightness update payload.
type BrightnessUpdate struct{ Value *statepb.Brightness }

// AudioVolumeUpdate retains an audio-volume update payload.
type AudioVolumeUpdate struct{ Value *statepb.AudioVolume }

// WiFiUpdate retains a Wi-Fi update payload.
type WiFiUpdate struct{ Value *statepb.Wifi }

// FirmwareUpdate retains a firmware-update state payload.
type FirmwareUpdate struct{ Value *updatepb.UpdateState }

// UpdateCheckUpdate retains a firmware-update check payload.
type UpdateCheckUpdate struct{ Value *updatepb.CheckState }

// TimezoneUpdate retains a timezone update payload.
type TimezoneUpdate struct{ Value *statepb.Timezone }

// MatterUpdate retains a Matter update payload.
type MatterUpdate struct{ Value *statepb.Matter }

// FrameUpdate retains a display-frame update payload.
type FrameUpdate struct{ Value *framepb.Frame }

// InputUpdate retains a device input event payload.
type InputUpdate struct{ Value *inputpb.InputEvent }

// TimerUpdate retains a timer update payload.
type TimerUpdate struct{ Value *timerpb.Timer }

// BLEUpdate retains a Bluetooth Low Energy update payload.
type BLEUpdate struct{ Value *blepb.Ble }

// AutoUpdateUpdate retains an automatic-update state payload.
type AutoUpdateUpdate struct{ Value *updatepb.AutoUpdateState }

// TimerProfilesUpdate retains a timer-profiles update payload.
type TimerProfilesUpdate struct{ Value *timerpb.Profiles }

// UnknownUpdate retains an unrecognized state-update payload.
type UnknownUpdate struct{ Value *statepb.StateUpdate }

// Kind identifies this payload as a device-name update.
func (DeviceNameUpdate) Kind() UpdateKind { return UpdateDeviceName }

// Kind identifies this payload as a power update.
func (PowerUpdate) Kind() UpdateKind { return UpdatePower }

// Kind identifies this payload as a brightness update.
func (BrightnessUpdate) Kind() UpdateKind { return UpdateBrightness }

// Kind identifies this payload as an audio-volume update.
func (AudioVolumeUpdate) Kind() UpdateKind { return UpdateAudioVolume }

// Kind identifies this payload as a Wi-Fi update.
func (WiFiUpdate) Kind() UpdateKind { return UpdateWiFi }

// Kind identifies this payload as a firmware-update state.
func (FirmwareUpdate) Kind() UpdateKind { return UpdateFirmware }

// Kind identifies this payload as an update-check state.
func (UpdateCheckUpdate) Kind() UpdateKind { return UpdateCheck }

// Kind identifies this payload as a timezone update.
func (TimezoneUpdate) Kind() UpdateKind { return UpdateTimezone }

// Kind identifies this payload as a Matter update.
func (MatterUpdate) Kind() UpdateKind { return UpdateMatter }

// Kind identifies this payload as a display-frame update.
func (FrameUpdate) Kind() UpdateKind { return UpdateFrame }

// Kind identifies this payload as an input event.
func (InputUpdate) Kind() UpdateKind { return UpdateInput }

// Kind identifies this payload as a timer update.
func (TimerUpdate) Kind() UpdateKind { return UpdateTimer }

// Kind identifies this payload as a Bluetooth Low Energy update.
func (BLEUpdate) Kind() UpdateKind { return UpdateBLE }

// Kind identifies this payload as an automatic-update state.
func (AutoUpdateUpdate) Kind() UpdateKind { return UpdateAutoUpdate }

// Kind identifies this payload as a timer-profiles update.
func (TimerProfilesUpdate) Kind() UpdateKind { return UpdateTimerProfiles }

// Kind identifies this payload as an unknown update.
func (UnknownUpdate) Kind() UpdateKind { return UpdateUnknown }

// Proto returns the retained device-name protobuf payload.
func (u DeviceNameUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained power protobuf payload.
func (u PowerUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained brightness protobuf payload.
func (u BrightnessUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained audio-volume protobuf payload.
func (u AudioVolumeUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained Wi-Fi protobuf payload.
func (u WiFiUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained firmware-update protobuf payload.
func (u FirmwareUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained update-check protobuf payload.
func (u UpdateCheckUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained timezone protobuf payload.
func (u TimezoneUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained Matter protobuf payload.
func (u MatterUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained display-frame protobuf payload.
func (u FrameUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained input-event protobuf payload.
func (u InputUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained timer protobuf payload.
func (u TimerUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained Bluetooth Low Energy protobuf payload.
func (u BLEUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained automatic-update protobuf payload.
func (u AutoUpdateUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained timer-profiles protobuf payload.
func (u TimerProfilesUpdate) Proto() proto.Message { return u.Value }

// Proto returns the retained unknown protobuf payload.
func (u UnknownUpdate) Proto() proto.Message { return u.Value }

// DeviceError is an error reported inside a firmware state message.
type DeviceError struct {
	Cause    errorpb.Cause
	Severity errorpb.Severity
	Raw      *errorpb.Error
}
