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

Create a client with a base URL that includes a scheme and host.

```go
client, err := busylib.NewClient(busylib.WithBaseURL("http://busybar.local"))
```

The client discovers the device API semantic version and sends it in `X-API-Sem-Ver`. It buffers response bodies only up to the configured maximum.

The retry policy can retry repeatable `GET`, `HEAD`, and `OPTIONS` requests after configured transient failures. It does not replay mutating methods. A call can still fail after partial network activity, so design mutations to be idempotent when the device API supports that contract.

## Local status stream

Use `NewStatusStream` to receive device status changes over WebSocket. A stream is one-shot: create it, start it once, and stop it when the caller is finished.

```go
statusStream, err := client.NewStatusStream()
if err != nil {
    return err
}
if err := statusStream.Start(ctx); err != nil {
    return err
}
defer statusStream.Stop()

for {
    select {
    case status := <-statusStream.Statuses():
        log.Printf("stream data status: %s", status.Data)
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

Cancel the context or call `Stop` to end receive activity. `Wait` returns the stable terminal or cleanup error after the stream has started or stopped.

## Remote MQTT

The `remote` package depends on a small `remote.Transport` interface. The caller supplies a connected implementation and remains responsible for its connection lifecycle.

```go
client, err := remote.NewClient(transport, "firmware-session", remote.WithClientID("example"))
if err != nil {
    return err
}

snapshot, err := client.Device().System().Status(ctx)
```

Remote payloads are decoded with a bounded message size. A subscription owns its internal receive path until it is closed or its context is canceled.

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
