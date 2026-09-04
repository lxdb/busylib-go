# Use the remote MQTT client

Use the `remote` package when the BUSY Bar is reachable through its MQTT 5 remote protocol instead of the local HTTP endpoint. The application supplies a connected `remote.Transport`; busylib maps typed service calls to firmware topics.

## Choose a transport

The optional [Eclipse Paho adapter](../../pahotransport/README.md) is the ready-to-use implementation. Applications can instead implement the small transport contract described in [Custom MQTT transport](custom-mqtt-transport.md).

Install the root module and, when needed, the Paho adapter:

```sh
go get github.com/lxdb/busylib-go@latest
go get github.com/lxdb/busylib-go/pahotransport@latest
```

## Connect with Eclipse Paho

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
		ClientID: "busybar-controller",
	},
})
if err != nil {
	return err
}

client, err := remote.NewClient(
	transport,
	"firmware-session",
	remote.WithClientID("controller"),
)
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

status, err := client.Device().System().Status(ctx)
if err != nil {
	return err
}
log.Printf("firmware version: %s", status.Firmware.Version)
```

The firmware session identifies the target device. The client ID identifies this remote client in request and response topics. Use values that match the broker and firmware configuration.

## Own the lifecycle

The caller owns the transport. `remote.Client.Close` closes the client's active subscriptions but does not close the MQTT connection. Close every remote client before closing their shared transport.

The Paho adapter restores active broker subscriptions after reconnection. The remote client continues to own its request correlation and response subscriptions during that transport lifecycle.

## Use supported services

Start from `client.Device()` and use the same typed services as the local client:

```go
device := client.Device()

name, err := device.Settings().Name(ctx)
if err != nil {
	return err
}

if err := device.Display().SetBrightness(ctx, "80"); err != nil {
	return err
}

log.Printf("device name: %s", name.Name)
```

Seven local operations are not available through the firmware remote protocol. The [service reference](../reference/services.md#remote-mqtt-restrictions) lists them from the checked protocol metadata.

## Receive status changes

Create a one-shot status stream from the remote client:

```go
statusStream, err := client.NewStatusStream()
if err != nil {
	return err
}
if err := statusStream.Start(ctx); err != nil {
	return err
}

for status := range statusStream.Statuses() {
	log.Printf("stream lifecycle=%s data=%s", status.Lifecycle, status.Data)
}
return statusStream.Wait()
```

The lifecycle rules match the local stream: start once, consume until the channel closes, and call `Wait` to observe the stable terminal result. Snapshot collection is not supported by the remote stream because the firmware remote protocol does not expose the local snapshot endpoint.

## Bound payloads

`remote.WithMaxMessageBytes` sets the maximum payload size for response and status-stream subscriptions. The default is `remote.DefaultMaxMessageBytes`. An oversized message fails with an error that is or wraps `remote.ErrMessageTooLarge`.

Use a bounded context for each request. Context cancellation bounds the caller's wait; the payload limit bounds memory retained for one message.
