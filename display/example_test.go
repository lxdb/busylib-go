package display_test

import (
	"fmt"

	"github.com/lxdb/busylib-go/display"
)

func ExampleTarget() {
	fmt.Println(display.Front)
	fmt.Println(display.Back)

	// Output:
	// front
	// back
}
