package busylib_test

import (
	"errors"
	"testing"

	"github.com/lxdb/busylib-go/frame"
	"github.com/lxdb/busylib-go/proto/framepb"
	"github.com/lxdb/busylib-go/remote"
	"github.com/lxdb/busylib-go/stream"
	"github.com/lxdb/busylib-go/usb"
)

func TestTransportErrorContracts(t *testing.T) {
	cause := errors.New("connection stopped")
	tests := []struct {
		err  error
		want string
	}{
		{&frame.Error{Operation: "pixels", Screen: framepb.Screen_FRONT, Encoding: framepb.Encoding_RUN_LENGTH, PixelFormat: framepb.PixelFormat_RGB888, Err: cause}, "BUSY Bar frame pixels failed (screen=0 encoding=1 pixel_format=0): connection stopped"},
		{&remote.Error{Operation: "publish", Route: "topic", Err: cause}, "remote publish topic failed: connection stopped"},
		{&remote.Error{Operation: "publish", Route: "topic"}, "remote publish topic failed"},
		{&stream.Error{Operation: "connect", Path: "/api/status/ws", StatusCode: 403}, "status stream connect /api/status/ws failed: HTTP 403"},
		{&stream.Error{Operation: "read", Path: "/api/status/ws", CloseCode: 1008, Err: cause}, "status stream read /api/status/ws failed: WebSocket close 1008: connection stopped"},
		{&stream.Error{Operation: "read", Path: "/api/status/ws", Err: cause}, "status stream read /api/status/ws failed: connection stopped"},
		{&stream.Error{Operation: "stop", Path: "/api/status/ws"}, "status stream stop /api/status/ws failed"},
		{&usb.Error{Operation: "send", Address: "10.0.4.20:23", Command: "uptime", Err: cause}, `USB CLI send 10.0.4.20:23 command "uptime" failed: connection stopped`},
		{&usb.Error{Operation: "dial", Address: "10.0.4.20:23", Err: cause}, "USB CLI dial 10.0.4.20:23 failed: connection stopped"},
	}
	for _, test := range tests {
		if got := test.err.Error(); got != test.want {
			t.Errorf("%T Error() = %q, want %q", test.err, got, test.want)
		}
		if unwrapper, ok := test.err.(interface{ Unwrap() error }); ok && unwrapper.Unwrap() != nil && !errors.Is(test.err, cause) {
			t.Errorf("%T did not preserve its cause", test.err)
		}
	}
}
