# Eclipse Paho MQTT transport

`pahotransport` is an optional Go module that adapts Eclipse Paho MQTT 5 to `github.com/lxdb/busylib-go/remote`. The separate module keeps a concrete MQTT implementation out of the root library.

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

client, err := remote.NewClient(transport, "firmware-session", remote.WithClientID("example"))
if err != nil {
	return errors.Join(err, transport.Close())
}
defer func() {
	clientErr := client.Close()
	transportErr := transport.Close()
	if err := errors.Join(clientErr, transportErr); err != nil {
		log.Printf("close remote MQTT client: %v", err)
	}
}()
```

Close every remote client before closing the transport that serves it.

## Subscription behavior

The adapter installs receive and reconnection callbacks before it connects. It preserves callbacks supplied in `autopaho.ClientConfig`, restores active subscriptions after reconnection, and shares one broker subscription among local subscribers to the same topic.

Each local subscription buffers at most 16 messages. A subscriber that fills its buffer terminates with `pahotransport.ErrSlowConsumer` without blocking Paho’s receive path. The maximum retained payload for that subscriber is 16 times its `remote.SubscriptionRequest.MaxPayloadBytes`, plus message metadata.

## Use remote services

Pass the connected transport to `remote.NewClient`, then start from `client.Device()`:

```go
status, err := client.Device().System().Status(ctx)
if err != nil {
	return err
}
log.Printf("firmware version: %s", status.Firmware.Version)
```

The root [Remote MQTT guide](../docs/integrations/remote-mqtt.md) covers service support, status streams, payload limits, and the ownership boundary. Close every remote client before closing the shared transport.

## Develop the module

Use the temporary workspace procedure in [Development](../docs/maintainers/development.md#work-across-all-modules) to test the adapter against the current root checkout. Do not commit a `go.work` file or local replacement. Before release, test the declared public dependency as described in [Releasing](../docs/maintainers/releasing.md#verify-the-declared-module-dependency).
