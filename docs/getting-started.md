# Getting started

This guide connects to a BUSY Bar on the local network, reads its status, and explains the error and upload contracts that callers must handle.

## Create a client

```go
// NewClient defaults to the BUSY Bar USB-network endpoint:
// http://10.0.4.20
client, err := busylib.NewClient()
if err != nil {
    return err
}

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

status, err := client.System().Status(ctx)
if err != nil {
    return err
}

log.Printf("firmware version: %s", status.Firmware.Version)
```

`NewClient()` uses `busylib.DefaultLocalBaseURL`, the BUSY Bar USB-network endpoint. To connect through another local-network address, pass `busylib.WithBaseURL("busybar.local")` or a complete HTTP or HTTPS URL. A missing scheme defaults to `http`; any path supplied in the base URL is discarded because the client stores only the endpoint origin.

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

## Prepare and inspect a request

Use `Prepare` when request construction and execution must happen separately.

```go
prepared, err := client.Prepare(busylib.Request{
    Method:       http.MethodGet,
    Path:         "/api/status",
    ResponseMode: busylib.ResponseModeJSON,
})
if err != nil {
    return err
}

targetURL := prepared.URL()
log.Printf(
    "request: %s %s request_id=%s",
    prepared.Method(),
    targetURL.String(),
    prepared.RequestID(),
)

response, err := client.DoPrepared(ctx, prepared)
if err != nil {
    return err
}

log.Printf("response status: %d", response.StatusCode)
```

`PreparedRequest` is immutable. Inspect it through `Method`, `Path`, `URL`, `Header`, `ResponseMode`, and `RequestID`. `URL` and `Header` return copies; changing those copies does not change later execution.

A prepared request can be executed more than once only when its body is repeatable. Prepare a new request instead of reusing one whose body is a one-shot stream.

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
