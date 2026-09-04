# Firmware updates

Use `UpdateService` to check available firmware, inspect update state, upload a local package, install a version, or configure automatic updates.

Firmware installation restarts or changes the device. Confirm the selected version and preserve another recovery path before starting an install.

## Check for updates

```go
if err := client.Update().Check(ctx); err != nil {
	return err
}

status, err := client.Update().Status(ctx)
if err != nil {
	return err
}
log.Printf("update status: %+v", status)
```

`Check` starts the device operation. Read `Status` or consume status-stream updates to observe progress and completion.

## Read a firmware changelog

```go
changelog, err := client.Update().Changelog(ctx, version)
if err != nil {
	return err
}
log.Printf("release notes: %s", changelog.Changelog)
```

Validate that the version is the one the application intends to install.

## Install a version

```go
if err := client.Update().Install(ctx, version); err != nil {
	return err
}
```

A successful request starts installation; it does not prove that the full update completed. Observe `Update().Status` or the typed firmware updates from a [status stream](status-streams.md).

Use `AbortDownload` only to stop an active download. It does not roll back firmware that is already installed.

## Upload a package

Package upload is local-only through the firmware MQTT proxy.

```go
if err := client.Update().UploadPackage(
	ctx,
	busylib.FileBody(packagePath, "application/octet-stream"),
); err != nil {
	return err
}
```

`FileBody` can reopen the file. The client still does not automatically retry this mutating operation.

## Configure automatic updates

Read the current settings before changing one field.

```go
settings, err := client.Update().AutoUpdate(ctx)
if err != nil {
	return err
}

enabled := true
settings.IsEnabled = &enabled
if err := client.Update().SetAutoUpdate(ctx, settings); err != nil {
	return err
}
```

A nil `IsEnabled` leaves the enabled state unchanged. Time-window values must satisfy the validation rules documented on [`AutoUpdateSettings`](https://pkg.go.dev/github.com/lxdb/busylib-go#AutoUpdateSettings).
