package busylib_test

import (
	"errors"
	"testing"

	busylib "github.com/lxdb/busylib-go"
)

func TestPublicErrorsPreserveDiagnosticsAndCauses(t *testing.T) {
	cause := errors.New("transport stopped")
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"API device message", &busylib.APIError{Method: "GET", Path: "/api/status", StatusCode: 403, RequestID: "rid", DeviceError: "denied"}, "GET /api/status failed: denied (status=403 request_id=rid)"},
		{"API status fallback", &busylib.APIError{Method: "GET", Path: "/api/status", StatusCode: 500}, "GET /api/status failed: HTTP 500 (status=500)"},
		{"request", &busylib.RequestError{Method: "GET", Path: "/api/status", Attempts: 2, RequestID: "rid", Err: cause}, "GET /api/status request failed after 2 attempt(s): transport stopped (request_id=rid)"},
		{"protocol cause", &busylib.ProtocolError{Method: "GET", Path: "/api/status", Err: cause}, "GET /api/status returned an invalid payload: transport stopped"},
		{"protocol fallback", &busylib.ProtocolError{Method: "GET", Path: "/api/status"}, "GET /api/status returned an invalid payload"},
		{"version message", &busylib.VersionError{Message: "unsupported API"}, "unsupported API"},
		{"version cause", &busylib.VersionError{Err: cause}, "API version negotiation failed: transport stopped"},
		{"version fallback", &busylib.VersionError{}, "API version negotiation failed"},
		{"validation message", &busylib.ValidationError{Message: "bad path"}, "bad path"},
		{"validation cause", &busylib.ValidationError{Err: cause}, "transport stopped"},
		{"validation fallback", &busylib.ValidationError{}, "invalid BUSY Bar request"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.err.Error(); got != test.want {
				t.Fatalf("Error() = %q, want %q", got, test.want)
			}
		})
	}

	for _, err := range []error{
		&busylib.RequestError{Err: cause},
		&busylib.ProtocolError{Err: cause},
		&busylib.VersionError{Err: cause},
		&busylib.ValidationError{Err: cause},
	} {
		if !errors.Is(err, cause) {
			t.Fatalf("%T did not preserve its cause", err)
		}
	}
}
