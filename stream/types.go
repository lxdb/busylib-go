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

// Stream is a one-shot local BUSY Bar status-stream subscription.
type Stream interface {
	Start(context.Context) error
	Stop() error
	RequestSnapshot(context.Context) error
	Messages() <-chan Message
	Statuses() <-chan Status
	Errors() <-chan error
	Status() Status
}

type MessageKind string

const (
	MessageBinary MessageKind = "binary"
	MessageText   MessageKind = "text"
)

// Message preserves one WebSocket message together with its decoded state and
// ordered product updates. Raw is retained for protocol diagnostics.
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

type Lifecycle string

const (
	LifecycleIdle         Lifecycle = "idle"
	LifecycleConnecting   Lifecycle = "connecting"
	LifecycleConnected    Lifecycle = "connected"
	LifecycleReconnecting Lifecycle = "reconnecting"
	LifecycleStopped      Lifecycle = "stopped"
	LifecycleFailed       Lifecycle = "failed"
)

type AccessStatus string

const (
	AccessUnknown  AccessStatus = "unknown"
	AccessAccepted AccessStatus = "accepted"
	AccessRejected AccessStatus = "rejected"
)

type DataStatus string

const (
	DataWaiting DataStatus = "waiting"
	DataFresh   DataStatus = "fresh"
	DataStale   DataStatus = "stale"
)

// Status is the current local stream lifecycle snapshot. Statuses may coalesce
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

type UpdateKind string

const (
	UpdateDeviceName    UpdateKind = "device_name"
	UpdatePower         UpdateKind = "power"
	UpdateBrightness    UpdateKind = "brightness"
	UpdateAudioVolume   UpdateKind = "audio_volume"
	UpdateWiFi          UpdateKind = "wifi"
	UpdateFirmware      UpdateKind = "update_state"
	UpdateCheck         UpdateKind = "update_check"
	UpdateTimezone      UpdateKind = "timezone"
	UpdateMatter        UpdateKind = "matter"
	UpdateFrame         UpdateKind = "frame"
	UpdateInput         UpdateKind = "input"
	UpdateTimer         UpdateKind = "timer"
	UpdateBLE           UpdateKind = "ble"
	UpdateAutoUpdate    UpdateKind = "auto_update_state"
	UpdateTimerProfiles UpdateKind = "timer_profiles"
	UpdateUnknown       UpdateKind = "unknown"
)

// Update classifies one firmware StateUpdate while retaining its generated
// protobuf payload and unknown enum values.
type Update interface {
	Kind() UpdateKind
	Proto() proto.Message
}

type DeviceNameUpdate struct{ Value *statepb.DeviceName }
type PowerUpdate struct{ Value *statepb.Power }
type BrightnessUpdate struct{ Value *statepb.Brightness }
type AudioVolumeUpdate struct{ Value *statepb.AudioVolume }
type WiFiUpdate struct{ Value *statepb.Wifi }
type FirmwareUpdate struct{ Value *updatepb.UpdateState }
type UpdateCheckUpdate struct{ Value *updatepb.CheckState }
type TimezoneUpdate struct{ Value *statepb.Timezone }
type MatterUpdate struct{ Value *statepb.Matter }
type FrameUpdate struct{ Value *framepb.Frame }
type InputUpdate struct{ Value *inputpb.InputEvent }
type TimerUpdate struct{ Value *timerpb.Timer }
type BLEUpdate struct{ Value *blepb.Ble }
type AutoUpdateUpdate struct{ Value *updatepb.AutoUpdateState }
type TimerProfilesUpdate struct{ Value *timerpb.Profiles }
type UnknownUpdate struct{ Value *statepb.StateUpdate }

func (DeviceNameUpdate) Kind() UpdateKind    { return UpdateDeviceName }
func (PowerUpdate) Kind() UpdateKind         { return UpdatePower }
func (BrightnessUpdate) Kind() UpdateKind    { return UpdateBrightness }
func (AudioVolumeUpdate) Kind() UpdateKind   { return UpdateAudioVolume }
func (WiFiUpdate) Kind() UpdateKind          { return UpdateWiFi }
func (FirmwareUpdate) Kind() UpdateKind      { return UpdateFirmware }
func (UpdateCheckUpdate) Kind() UpdateKind   { return UpdateCheck }
func (TimezoneUpdate) Kind() UpdateKind      { return UpdateTimezone }
func (MatterUpdate) Kind() UpdateKind        { return UpdateMatter }
func (FrameUpdate) Kind() UpdateKind         { return UpdateFrame }
func (InputUpdate) Kind() UpdateKind         { return UpdateInput }
func (TimerUpdate) Kind() UpdateKind         { return UpdateTimer }
func (BLEUpdate) Kind() UpdateKind           { return UpdateBLE }
func (AutoUpdateUpdate) Kind() UpdateKind    { return UpdateAutoUpdate }
func (TimerProfilesUpdate) Kind() UpdateKind { return UpdateTimerProfiles }
func (UnknownUpdate) Kind() UpdateKind       { return UpdateUnknown }

func (u DeviceNameUpdate) Proto() proto.Message    { return u.Value }
func (u PowerUpdate) Proto() proto.Message         { return u.Value }
func (u BrightnessUpdate) Proto() proto.Message    { return u.Value }
func (u AudioVolumeUpdate) Proto() proto.Message   { return u.Value }
func (u WiFiUpdate) Proto() proto.Message          { return u.Value }
func (u FirmwareUpdate) Proto() proto.Message      { return u.Value }
func (u UpdateCheckUpdate) Proto() proto.Message   { return u.Value }
func (u TimezoneUpdate) Proto() proto.Message      { return u.Value }
func (u MatterUpdate) Proto() proto.Message        { return u.Value }
func (u FrameUpdate) Proto() proto.Message         { return u.Value }
func (u InputUpdate) Proto() proto.Message         { return u.Value }
func (u TimerUpdate) Proto() proto.Message         { return u.Value }
func (u BLEUpdate) Proto() proto.Message           { return u.Value }
func (u AutoUpdateUpdate) Proto() proto.Message    { return u.Value }
func (u TimerProfilesUpdate) Proto() proto.Message { return u.Value }
func (u UnknownUpdate) Proto() proto.Message       { return u.Value }

// DeviceError is an error reported inside a firmware state message.
type DeviceError struct {
	Cause    errorpb.Cause
	Severity errorpb.Severity
	Raw      *errorpb.Error
}
