# Client reference

`busylib.Client` sends HTTP requests to one BUSY Bar endpoint. It is safe for concurrent use. Callers own operation contexts and must handle every returned error.

## Create a client

```go
client, err := busylib.NewClient(
	busylib.WithBaseURL("busybar.local"),
	busylib.WithLocalAccessToken(accessToken),
)
if err != nil {
	return err
}
```

`NewClient()` uses `busylib.DefaultLocalBaseURL`. A base URL without a scheme uses `http`. A supplied path is discarded because the client stores only the endpoint origin.

## Client options

| Option | Use |
| --- | --- |
| [`WithBaseURL`](https://pkg.go.dev/github.com/lxdb/busylib-go#WithBaseURL) | Select a hostname, IP address, or HTTP or HTTPS origin. |
| [`WithHTTPClient`](https://pkg.go.dev/github.com/lxdb/busylib-go#WithHTTPClient) | Supply the `http.Client` used for all requests. |
| [`WithTimeout`](https://pkg.go.dev/github.com/lxdb/busylib-go#WithTimeout) | Limit a request when its context has no earlier deadline. Zero disables the client timeout. |
| [`WithLocalAccessToken`](https://pkg.go.dev/github.com/lxdb/busylib-go#WithLocalAccessToken) | Send a numeric access key or minted access token. Preferred for new code. Remote mode rejects this option. |
| [`WithLocalAccessKey`](https://pkg.go.dev/github.com/lxdb/busylib-go#WithLocalAccessKey) | Deprecated source-compatible alias for `WithLocalAccessToken`. |
| [`WithSessionID`](https://pkg.go.dev/github.com/lxdb/busylib-go#WithSessionID) | Add a stable session identifier to requests. |
| [`WithRequestIDGenerator`](https://pkg.go.dev/github.com/lxdb/busylib-go#WithRequestIDGenerator) | Replace the concurrent-safe request ID generator. |
| [`WithRetryPolicy`](https://pkg.go.dev/github.com/lxdb/busylib-go#WithRetryPolicy) | Set attempts and backoff for safe, repeatable requests. |
| [`WithVersionNegotiation`](https://pkg.go.dev/github.com/lxdb/busylib-go#WithVersionNegotiation) | Enable or disable firmware API version discovery. |
| [`WithMaxResponseBytes`](https://pkg.go.dev/github.com/lxdb/busylib-go#WithMaxResponseBytes) | Change the maximum response size buffered in memory. |
| [`WithEndpointMode`](https://pkg.go.dev/github.com/lxdb/busylib-go#WithEndpointMode) | Select local or remote request rules. Applications normally receive remote mode through `remote.NewClient`. |

Options validate their input during `NewClient`. An invalid option returns an error before the client sends a request.

`WithLocalAccessToken` and the deprecated `WithLocalAccessKey` both send the credential in `X-API-Token`. Do not log the credential, the header, or a response payload that contains a token.

## Use typed services

Typed services validate input, construct the firmware request, and decode the result.

```go
name, err := client.Settings().Name(ctx)
if err != nil {
	return err
}
log.Printf("device name: %s", name.Name)
```

Use the [service reference](services.md) to find all service methods. Prefer a typed service over a low-level request when the service supports the operation.

## Send a low-level request

Use `Do` when a firmware operation is not exposed through a typed service.

```go
response, err := client.Do(ctx, busylib.Request{
	Method:       http.MethodGet,
	Path:         "/api/status",
	ResponseMode: busylib.ResponseModeJSON,
})
if err != nil {
	return err
}

var status busylib.Status
if err := response.DecodeJSON(&status); err != nil {
	return err
}
```

`Request.Path` must identify an API path. The client normalizes the method, query, headers, session ID, request ID, body, and response mode before transport use.

| Response mode | Contract |
| --- | --- |
| `ResponseModeJSON` | Buffer the response and reject invalid JSON. |
| `ResponseModeBytes` | Buffer arbitrary bytes. |
| `ResponseModeText` | Buffer text bytes without JSON validation. |

The configured maximum response size applies to all buffered modes. Use `Storage().ReadTo` for a large storage file.

## Prepare a request

Use `Prepare` when request construction and execution happen at different times.

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

`PreparedRequest` is immutable. `URL` returns a value copy, and `Header` returns a cloned map. Changing either returned value does not change later execution.

Repeated execution is safe only when the body is repeatable. Prepare a new request instead of reusing a one-shot reader body.

## Choose a request body

| Constructor | Repeatable | Use |
| --- | --- | --- |
| [`JSONBody`](https://pkg.go.dev/github.com/lxdb/busylib-go#JSONBody) | Yes | Encode a Go value as JSON. |
| [`BytesBody`](https://pkg.go.dev/github.com/lxdb/busylib-go#BytesBody) | Yes | Copy and send an in-memory byte slice. |
| [`FileBody`](https://pkg.go.dev/github.com/lxdb/busylib-go#FileBody) | Yes | Open a local file for each attempt. |
| [`RepeatableBody`](https://pkg.go.dev/github.com/lxdb/busylib-go#RepeatableBody) | Yes | Open a fresh reader for each attempt. |
| [`ReaderBody`](https://pkg.go.dev/github.com/lxdb/busylib-go#ReaderBody) | No | Send one reader once. The client closes it when it implements `io.ReadCloser`. |
| [`ProgressBody`](https://pkg.go.dev/github.com/lxdb/busylib-go#ProgressBody) | Inherited | Report bytes read while preserving the wrapped body's replay rules. |

`ProgressFunc` receives progress for the current attempt. A retry starts the count again at zero. The total is `-1` when the body length is unknown.

## Understand retries

Automatic transport retries require both conditions:

1. The method is `GET`, `HEAD`, or `OPTIONS`.
2. The request body is repeatable.

The retry policy never automatically replays a mutating method. A compatibility response can cause one API-version refresh and retry, but only when the body can be opened again.

A failed call can follow partial network activity. Use a firmware operation's idempotency contract when the application must decide whether to retry a mutation.

## Correlate requests

The client creates an `X-Request-ID` when the caller does not supply one. `Response.RequestID` and structured errors retain request context. Use `WithSessionID` or `Request.SessionID` when several operations must share a session identifier.

Do not log access keys, authorization headers, MQTT credentials, or response payloads that can contain private data.
