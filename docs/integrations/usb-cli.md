# Use the USB CLI

The `usb` package accesses the raw firmware command line through the BUSY Bar USB-network interface. It is independent of the HTTP client and status stream.

## Requirements

The host must have a route to the USB-network interface, and the device firmware must expose the CLI. The default address is `usb.DefaultAddress` (`10.0.4.20:23`).

Create a client and run one bounded command:

```go
client, err := usb.NewClient()
if err != nil {
    return err
}

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

response, err := client.Commands().Uptime(ctx)
if err != nil {
    return err
}
log.Print(response.Output)
```

Use `usb.WithAddress` for another CLI address. Options also control dial timeout, command timeout, and maximum response size.

## Choose a connection lifetime

Direct client operations open a fresh connection for each command. This is the simplest choice for isolated operations:

```go
response, err := client.Commands().DeviceInfo(ctx)
```

Open a persistent session when several commands should use one connection:

```go
session, err := client.Open(ctx)
if err != nil {
    return err
}
defer session.Close()

uptime, err := session.Commands().Uptime(ctx)
if err != nil {
    return err
}
free, err := session.Commands().Free(ctx)
if err != nil {
    return err
}

log.Printf("uptime=%q free=%q", uptime.Output, free.Output)
```

A session serializes commands. If a transport error occurs or the prompt cannot be recovered, the session closes and cannot be reused. Open a new session explicitly.

## Use command wrappers

`Commands` provides wrappers for the supported firmware commands: uptime, power, storage, update, input, loader, top, free, free-blocks, log, echo, device information, date, timezone, Matter, audio, display, sysctl, log dump, and reboot.

Use `SendCommand` only when the required firmware command does not have a wrapper. Arguments are validated and encoded as a CLI command line; do not build an untrusted command string yourself.

## Stream continuous output

`Top` and `Log` write continuous output to an `io.Writer`. Cancel the context to stop the command:

```go
streamCtx, stop := context.WithTimeout(ctx, 10*time.Second)
defer stop()

err := client.Commands().Top(streamCtx, os.Stdout, time.Second)
if err != nil && !errors.Is(err, context.DeadlineExceeded) {
    return err
}
```

Cancellation sends the firmware ETX interrupt byte and waits for the prompt. A persistent session remains usable only if that recovery succeeds. A fresh-connection stream closes its connection after completion.

## Interpret failures

A connection failure can mean that host routing, permissions, the address, or the firmware CLI is unavailable. It does not by itself identify a library defect. Use `errors.Is` with context errors and USB sentinel errors, and use `errors.As` to inspect `*usb.Error` for the operation, address, and command.
