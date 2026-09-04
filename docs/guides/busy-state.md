# Busy state

Use `BusyService` to read or replace the active timer snapshot and saved busy profiles.

## Read the active snapshot

```go
current, err := client.Busy().Snapshot(ctx)
if err != nil {
	return err
}

log.Printf(
	"busy state=%s timestamp_ms=%d",
	current.Snapshot.Type,
	current.SnapshotTimestampMS,
)
```

The snapshot contains the active timer state and Busy Bar settings. Pointer fields distinguish an omitted firmware value from a zero value.

## Replace the active snapshot

`SetSnapshot` replaces the current busy state. Read the current snapshot first when the application intends to change only one field.

```go
current, err := client.Busy().Snapshot(ctx)
if err != nil {
	return err
}

paused := true
if current.Snapshot.Type == busylib.BusySnapshotNotStarted {
	return errors.New("cannot pause a timer that has not started")
}
current.Snapshot.IsPaused = &paused
if err := client.Busy().SetSnapshot(ctx, current); err != nil {
	return err
}
```

The library validates timer and Busy Bar settings before it sends the update. Validation does not know which fields your application owns. Preserve every field that you do not intend to change.

## Read a saved profile

```go
profile, err := client.Busy().Profile(ctx, busylib.BusyProfileSlotBusy)
if err != nil {
	return err
}
log.Printf("profile: %s", profile.Title)
```

The supported built-in slots are defined by [`busylib.BusyProfileSlot`](https://pkg.go.dev/github.com/lxdb/busylib-go#BusyProfileSlot).

## Replace a saved profile

`SetProfile` replaces the complete profile in the selected slot.

```go
profile, err := client.Busy().Profile(ctx, busylib.BusyProfileSlotBusy)
if err != nil {
	return err
}

profile.Title = "Deep work"
if err := client.Busy().SetProfile(
	ctx,
	busylib.BusyProfileSlotBusy,
	profile,
); err != nil {
	return err
}
```

Read, modify, and validate a current profile when the application must preserve settings it does not manage.
