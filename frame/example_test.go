package frame_test

import (
	"fmt"

	busylib "github.com/lxdb/busylib-go"
	"github.com/lxdb/busylib-go/frame"
)

func ExampleFromHTTP() {
	raw := make([]byte, frame.FrontWidth*frame.FrontHeight*3)
	value, err := frame.FromHTTP(busylib.DisplayFront, raw)
	if err != nil {
		return
	}
	decoded, err := value.RGBA()
	if err != nil {
		return
	}
	fmt.Println(decoded.Bounds())
	// Output:
	// (0,0)-(72,16)
}
