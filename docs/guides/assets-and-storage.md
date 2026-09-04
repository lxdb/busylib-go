# Assets and storage

Use application assets for display and audio content. Use storage methods for direct file and directory operations.

## Upload an application asset

An asset belongs to one application name and one relative file path.

```go
if err := client.Assets().UploadFile(
	ctx,
	"calendar",
	"logo.png",
	"./logo.png",
); err != nil {
	return err
}
```

Use `Assets().Upload` with `BytesBody`, `ReaderBody`, `FileBody`, or another body constructor when the source is not a local file.

Delete all assets for an application only when the caller owns that namespace:

```go
if err := client.Assets().DeleteApplicationAssets(ctx, "calendar"); err != nil {
	return err
}
```

This operation removes every asset owned by `calendar`. It does not remove only one file.

## Write a storage file

```go
if err := client.Storage().WriteFile(
	ctx,
	"/ext/calendar/cache.json",
	"./cache.json",
); err != nil {
	return err
}
```

`WriteFile` uses a repeatable file body. `Storage().Write` accepts any `busylib.Body` and preserves that body's replay rules.

## Read a small file

`Read` buffers the complete file and applies the client's maximum response size.

```go
data, err := client.Storage().Read(ctx, "/ext/calendar/cache.json")
if err != nil {
	return err
}
log.Printf("read %d bytes", len(data))
```

## Stream a large file

Use `ReadTo` when a file should not be buffered in memory.

```go
destination, err := os.Create("device.log")
if err != nil {
	return err
}
defer destination.Close()

written, err := client.Storage().ReadTo(
	ctx,
	"/ext/logs/device.log",
	destination,
)
if err != nil {
	return err
}
log.Printf("wrote %d bytes", written)
```

The caller owns the destination writer and must close it.

## Manage directories

```go
if err := client.Storage().Mkdir(ctx, "/ext/calendar"); err != nil {
	return err
}

entries, err := client.Storage().List(ctx, "/ext/calendar")
if err != nil {
	return err
}

if err := client.Storage().Rename(
	ctx,
	"/ext/calendar/cache.json",
	"/ext/calendar/cache.previous.json",
); err != nil {
	return err
}
```

`Remove` deletes the selected file or directory. Confirm the validated device path before calling it. The library validates path shape, but only the application knows whether the target should be deleted.

## Check capacity

```go
status, err := client.Storage().Status(ctx)
if err != nil {
	return err
}
log.Printf("storage: %+v", status)
```

Check capacity before a large upload when the application can choose a smaller asset or clean up its own files.
