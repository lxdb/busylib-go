package snapshot_test

import (
	"fmt"

	"github.com/lxdb/busylib-go/proto/statepb"
	"github.com/lxdb/busylib-go/snapshot"
	"github.com/lxdb/busylib-go/stream"
)

func ExampleStore() {
	store := snapshot.NewStore(snapshot.Snapshot{})
	change := store.Apply(stream.DeviceNameUpdate{
		Value: &statepb.DeviceName{Name: "Studio"},
	})
	current := store.Snapshot()

	fmt.Println(change.Sections)
	fmt.Println(current.Name.Value)

	// Output:
	// [name]
	// Studio
}
