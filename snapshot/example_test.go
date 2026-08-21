package snapshot_test

import (
	"github.com/lxdb/busylib-go/snapshot"
	"github.com/lxdb/busylib-go/stream"
)

func ExampleStore() {
	store := snapshot.NewStore(snapshot.Snapshot{})
	change := store.Apply(stream.DeviceNameUpdate{})
	_ = change.Sections
	current := store.Snapshot()
	_ = current
}
