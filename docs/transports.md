# Transports

Choose the transport by where the device is reachable and by who must own the connection lifecycle.

| Transport | Use it when | Caller owns | Library owns |
| --- | --- | --- | --- |
| Local HTTP | The device API is reachable on the local network | Contexts and client use | Request construction, bounded response handling, and safe retry decisions |
| Local WebSocket status | The device status endpoint is reachable on the local network | Stream cancellation or close | Dialing, decoding, and stream shutdown |
| Remote MQTT | The caller already has an MQTT transport | Transport connection and shutdown | Topic mapping, payload decoding, and subscription lifecycle |
| Paho adapter | Eclipse Paho is the selected MQTT implementation | Adapter connection and shutdown | Paho configuration bridge and subscription delivery |
| USB CLI | A host reaches the raw device CLI through the USB network interface | Contexts, address selection, and client use | TCP session, prompt handling, response bounds, and command recovery |

## Local HTTP

Create a client with the default USB-network endpoint, or pass an alternate hostname or HTTP or HTTPS URL.

```go
client, err := busylib.NewClient(busylib.WithBaseURL("http://busybar.local"))
```

The client discovers the device API semantic version and sends it in `X-API-Sem-Ver`. It buffers response bodies only up to the configured maximum.

The retry policy can retry repeatable `GET`, `HEAD`, and `OPTIONS` requests after configured transient failures. It does not replay mutating methods. A call can still fail after partial network activity, so design mutations to be idempotent when the device API supports that contract.

## Local status stream

Use `NewStatusStream` to receive device status changes over WebSocket. A stream is one-shot: it can be started once, and `Wait` exposes its stable completion result.

```go
statusStream, err := client.NewStatusStream()
if err != nil {
    return err
}
if err := statusStream.Start(ctx); err != nil {
    return err
}

statuses := statusStream.Statuses()
for {
    select {
    case status, ok := <-statuses:
        if !ok {
            return statusStream.Wait()
        }
        log.Printf(
            "stream lifecycle=%s data=%s",
            status.Lifecycle,
            status.Data,
        )

    case <-ctx.Done():
        return errors.Join(ctx.Err(), statusStream.Stop())
    }
}
```

After `Start` succeeds, `Wait` blocks until the stream finishes and then returns its terminal or cleanup error. Repeated calls return the same result. `Stop` requests shutdown, waits for cleanup, and returns the same completion result. Calling `Wait` before either `Start` or `Stop` returns `stream.ErrNotStarted`. A stream cannot be restarted after it finishes.

## Remote MQTT

The `remote` package depends on a small `remote.Transport` interface. The caller supplies a connected implementation and remains responsible for its connection lifecycle.

```go
client, err := remote.NewClient(transport, "firmware-session", remote.WithClientID("example"))
if err != nil {
    return err
}

snapshot, err := client.Device().System().Status(ctx)
```

`remote.Client` supplies `SubscriptionRequest.MaxPayloadBytes` on every HTTP-response and status-stream subscription. A transport implementation must:

- reject a non-positive `MaxPayloadBytes` value when subscribing;
- check each received payload before retaining or copying it for that subscriber;
- stop delivery to the subscriber when a payload exceeds the requested limit and make `Receive` return an error that is or wraps `remote.ErrMessageTooLarge`; and
- unblock an outstanding `Receive` when `Subscription.Close` is called.

This lets callers identify an oversized payload with `errors.Is`.

The requested limit comes from `remote.WithMaxMessageBytes`; its default is `remote.DefaultMaxMessageBytes`. The caller still owns the MQTT connection and must close it only after all remote clients that use it have been closed.

## Eclipse Paho adapter

The optional `pahotransport` module adapts Eclipse Paho to `remote.Transport`. It is a separate Go module so that the root library does not require a specific MQTT implementation.

```go
broker, err := url.Parse("mqtt://broker.example:1883")
if err != nil {
    return err
}

adapter, err := pahotransport.Dial(ctx, autopaho.ClientConfig{
    ServerUrls: []*url.URL{broker},
    ClientConfig: paho.ClientConfig{
        ClientID: "busybar-controller",
    },
})
if err != nil {
    return err
}
defer adapter.Close()

client, err := remote.NewClient(adapter, "firmware-session", remote.WithClientID("example"))
```

See the [Paho adapter README](../pahotransport/README.md) for module requirements and a complete lifecycle example.

## USB CLI

The `usb` package connects to the raw firmware CLI through the USB network interface. It is independent of the HTTP and WebSocket APIs.

```go
client, err := usb.NewClient(usb.WithAddress("10.0.4.20:23"))
if err != nil {
    return err
}

uptime, err := client.Commands().Uptime(ctx)
```

The USB package talks to the device CLI through the USB network interface. Host routing, permissions, and the firmware CLI must all be available; a connection failure does not by itself identify which boundary is unavailable.
