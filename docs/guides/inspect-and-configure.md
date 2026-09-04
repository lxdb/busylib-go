# Inspect and configure a device

Use the system, settings, and time services to inspect a device before changing its configuration.

## Read the current state

Start with the aggregate status when the application needs a general device view:

```go
status, err := client.System().Status(ctx)
if err != nil {
	return err
}

log.Printf(
	"serial=%s firmware=%s power=%s",
	status.Device.SerialNumber,
	status.Firmware.Version,
	status.Power.State,
)
```

Use focused methods when the application needs one section and should not depend on the aggregate response:

```go
device, err := client.System().DeviceStatus(ctx)
if err != nil {
	return err
}

power, err := client.System().PowerStatus(ctx)
if err != nil {
	return err
}

network, err := client.System().Transport(ctx)
if err != nil {
	return err
}
```

The [system service reference](../reference/services.md#system) lists version, firmware, runtime, power, transport, and log methods.

## Change the device name

Read the current value before changing it when the application needs conditional behavior.

```go
current, err := client.Settings().Name(ctx)
if err != nil {
	return err
}
if current.Name != "Focus room" {
	if err := client.Settings().SetName(ctx, "Focus room"); err != nil {
		return err
	}
}
```

`SetName` validates the value before sending a request.

## Change HTTP access

`SetHTTPAccess` changes how clients can reach the local device API. An incorrect mode or key can prevent later local requests.

```go
current, err := client.Settings().HTTPAccess(ctx)
if err != nil {
	return err
}
log.Printf("current HTTP access mode: %s", current.Mode)

if err := client.Settings().SetHTTPAccess(
	ctx,
	busylib.HTTPAccessKey,
	accessKey,
); err != nil {
	return err
}
```

Create later local clients with `busylib.WithLocalAccessKey(accessKey)`. Do not log the key.

## Read or set time

```go
now, err := client.Time().Now(ctx)
if err != nil {
	return err
}

timezone, err := client.Time().Timezone(ctx)
if err != nil {
	return err
}

log.Printf("device time=%s timezone=%s", now.Timestamp, timezone.Name)
```

Before setting a timezone, use `client.Time().Timezones(ctx)` to obtain names accepted by the device. `SetTimestamp` requires an RFC 3339 timestamp.

## Create a diagnostic archive

`DumpLog` creates an archive in device storage. An empty filename lets the device select the archive name.

```go
result, err := client.System().DumpLog(ctx, "")
if err != nil {
	return err
}
log.Printf("device log archive: %s", result.Path)
```

Use [Assets and storage](assets-and-storage.md) to read or remove the archive. Treat log data as potentially sensitive.
