package convert_test

import (
	"errors"
	"testing"

	"github.com/lxdb/busylib-go/convert"
)

func TestConversionErrorContract(t *testing.T) {
	cause := errors.New("decode stopped")
	tests := []struct {
		err  *convert.ConversionError
		want string
	}{
		{&convert.ConversionError{Operation: "decode", Format: "png", Err: cause}, "image decode failed for png input: decode stopped"},
		{&convert.ConversionError{Operation: "read", Err: cause}, "image read failed: decode stopped"},
	}
	for _, test := range tests {
		if got := test.err.Error(); got != test.want {
			t.Fatalf("Error() = %q, want %q", got, test.want)
		}
		if !errors.Is(test.err, cause) {
			t.Fatal("ConversionError did not preserve its cause")
		}
	}
}
