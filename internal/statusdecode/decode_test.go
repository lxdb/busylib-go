package statusdecode

import (
	"errors"
	"reflect"
	"testing"

	"github.com/lxdb/busylib-go/proto/errorpb"
	"github.com/lxdb/busylib-go/proto/statepb"
	publicstream "github.com/lxdb/busylib-go/stream"
	"google.golang.org/protobuf/proto"
)

func TestDecodeBinaryPreservesAndMapsFirmwareState(t *testing.T) {
	payload, err := proto.Marshal(&statepb.State{
		Timestamp: 42,
		Updates: []*statepb.StateUpdate{
			{State: &statepb.StateUpdate_DeviceName{DeviceName: &statepb.DeviceName{Name: "BUSY Bar"}}},
			{State: &statepb.StateUpdate_Power{Power: &statepb.Power{}}},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	message, fatal := DecodeBinary(payload, "sessions/session/up/v1/stream-response/client")
	if fatal || message.DecodeError != nil {
		t.Fatalf("message = %#v, fatal=%v", message, fatal)
	}
	if !reflect.DeepEqual(message.Raw, payload) || message.State.GetTimestamp() != 42 {
		t.Fatalf("decoded state = %#v", message.State)
	}
	if got := []publicstream.UpdateKind{message.Updates[0].Kind(), message.Updates[1].Kind()}; !reflect.DeepEqual(got, []publicstream.UpdateKind{publicstream.UpdateDeviceName, publicstream.UpdatePower}) {
		t.Fatalf("update kinds = %#v", got)
	}
}

func TestDecodeBinaryReturnsProtocolAndFatalDeviceErrors(t *testing.T) {
	malformed, fatal := DecodeBinary([]byte{0xff}, "remote-topic")
	var protocolErr *publicstream.Error
	if fatal || !errors.As(malformed.DecodeError, &protocolErr) || protocolErr.Path != "remote-topic" {
		t.Fatalf("malformed message = %#v, fatal=%v", malformed, fatal)
	}

	payload, err := proto.Marshal(&statepb.State{Error: &errorpb.Error{
		Cause:    errorpb.Cause_RESOURCE_LIMIT,
		Severity: errorpb.Severity_FATAL,
	}})
	if err != nil {
		t.Fatalf("marshal fatal error: %v", err)
	}
	message, fatal := DecodeBinary(payload, "remote-topic")
	if !fatal || message.DeviceError == nil || message.DeviceError.Severity != errorpb.Severity_FATAL {
		t.Fatalf("device error = %#v, fatal=%v", message.DeviceError, fatal)
	}
}
