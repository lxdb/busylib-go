# Getting started

This guide connects to a BUSY Bar on the local network, reads its status, and explains the error and upload contracts that callers must handle.

## Create a client

`NewClient` validates the base URL and creates a local HTTP client. Give every request a deadline so that unreachable devices do not block the caller indefinitely.

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

client, err := busylib.NewClient(busylib.WithBaseURL("http://busybar.local"))
if err != nil {
    return err
}

status, err := client.System().Status(ctx)
if err != nil {
    return err
}

log.Printf("firmware version: %s", status.Firmware.Version)
```

The endpoint can use a DNS name or an IP address. Include the scheme. The client rejects a base URL without a host.

## Control API version negotiation

The client discovers the device API semantic version from `/api/version`, caches a successful response, and sends it in the `X-API-Sem-Ver` header. A compatibility response can cause one refresh and retry when the request is safe to repeat.

```go
client, err := busylib.NewClient(
    busylib.WithBaseURL("http://busybar.local"),
    busylib.WithVersionNegotiation(busylib.VersionNegotiationDisabled),
)
```

Disable negotiation only when the endpoint does not implement the version contract. The client then omits version discovery and the version header.

## Handle API errors

HTTP operations return `*busylib.APIError` for non-success responses. Use `errors.As` when code must inspect the status code or parsed device message.

```go
var apiErr *busylib.APIError
if errors.As(err, &apiErr) {
    log.Printf("device request failed: status=%d message=%q", apiErr.StatusCode, apiErr.DeviceError)
}
```

Error bodies are bounded. If a response exceeds the configured limit, the client returns a size error instead of buffering the entire body.

## Stream large files

Use the storage streaming methods when a file should not be buffered in memory.

```go
err := client.Storage().WriteFile(ctx, "/ext/example.bin", localPath)
```

`Storage().ReadTo` streams a device file to an `io.Writer`. The built-in retry policy automatically retries only repeatable `GET`, `HEAD`, and `OPTIONS` requests. It does not replay mutating requests.

## Continue by task

- Read [Transports](transports.md) before adding a status stream, MQTT connection, or USB CLI operation.
- Read [Media](media.md) before sending images, audio, animations, or raw display frames.
- Read [Compatibility](compatibility.md) before changing firmware, toolchain, or resource-limit assumptions.
