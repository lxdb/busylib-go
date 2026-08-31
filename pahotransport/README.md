# Eclipse Paho transport

`pahotransport` is an optional Go module that adapts Eclipse Paho MQTT 5 to `github.com/lxdb/busylib-go/remote`. The separate module keeps a concrete MQTT implementation out of the root library.

> [!WARNING]
> This module cannot be published before the root module’s protobuf redistribution blocker is resolved. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## Requirements

Read `go.mod` for this module’s Go version and minimum root-module requirement. When the root requirement changes, update it in the same change and test the declared dependency with workspaces disabled after that root version is public.

## Connect

`Dial` creates and connects a transport. Its context bounds the initial connection attempt. `Close` ends the transport lifetime.

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

broker, err := url.Parse("mqtt://broker.example:1883")
if err != nil {
    return err
}

transport, err := pahotransport.Dial(ctx, autopaho.ClientConfig{
    ServerUrls: []*url.URL{broker},
    ClientConfig: paho.ClientConfig{
        ClientID: "busylib-example",
    },
})
if err != nil {
    return err
}
defer transport.Close()

client, err := remote.NewClient(transport, "firmware-session", remote.WithClientID("example"))
if err != nil {
    return err
}
defer client.Close()
```

Close every remote client before closing the transport that serves it.

## Subscription behavior

The adapter installs receive and reconnection callbacks before it connects. It preserves callbacks supplied in `autopaho.ClientConfig`, restores active subscriptions after reconnection, and shares one broker subscription among local subscribers to the same topic.

Each local subscription buffers at most 16 messages. A subscriber that fills its buffer terminates with `pahotransport.ErrSlowConsumer` without blocking Paho’s receive path. The maximum retained payload for that subscriber is 16 times its `remote.SubscriptionRequest.MaxPayloadBytes`, plus message metadata.

## Develop the module

Use the temporary workspace procedure in [Development](../docs/development.md) to test the adapter against the current root checkout. Do not commit a `go.work` file or local replacement.
