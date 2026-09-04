# Status streams

Status streams deliver live device updates over a local WebSocket or remote MQTT connection. Both transports implement the same one-shot `stream.Stream` lifecycle.

## Consume a local stream

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
			"stream lifecycle=%s data=%s attempt=%d",
			status.Lifecycle,
			status.Data,
			status.Attempt,
		)

	case <-ctx.Done():
		return errors.Join(ctx.Err(), statusStream.Stop())
	}
}
```

Read the channel with the two-value receive form. A closed channel otherwise returns zero values forever and can create a busy loop.

## Lifecycle contract

| Method | Contract |
| --- | --- |
| `Start(ctx)` | Open the stream once. A second call returns `stream.ErrAlreadyStarted`. |
| `Stop()` | Request shutdown, wait for cleanup, and return the stable completion result. It is valid before `Start`. |
| `Wait()` | Block until completion after `Start` or `Stop`. Before either call, return `stream.ErrNotStarted`. |
| `Status()` | Return the latest lifecycle snapshot. |
| `Statuses()` | Deliver lifecycle snapshots. Intermediate values can coalesce. |
| `Messages()` | Deliver decoded messages and ordered typed updates. |
| `RequestSnapshot(ctx)` | Ask a local stream for an immediate snapshot. Remote MQTT returns `stream.ErrSnapshotUnsupported`. |

A stream cannot restart after it finishes. Create a new stream for another connection lifetime. Repeated `Wait` calls return the same terminal or cleanup result.

## Consume device updates

```go
messages := statusStream.Messages()
for {
	select {
	case message, ok := <-messages:
		if !ok {
			return statusStream.Wait()
		}
		if message.DecodeError != nil {
			log.Printf("discard malformed stream message: %v", message.DecodeError)
			continue
		}
		for _, update := range message.Updates {
			log.Printf("device update: %s", update.Kind())
		}

	case <-ctx.Done():
		return errors.Join(ctx.Err(), statusStream.Stop())
	}
}
```

`Message.Raw` retains protocol bytes for diagnostics. Do not log raw messages unless the application has reviewed their data sensitivity.

## Maintain a current snapshot

Collect an initial best-effort snapshot, then apply ordered stream updates.

```go
initial, err := snapshot.Collect(ctx, client)
if err != nil {
	return err
}

store := snapshot.NewStore(initial)
messages := statusStream.Messages()
for message := range messages {
	if message.DecodeError != nil {
		continue
	}
	change := store.Apply(message.Updates...)
	if len(change.Sections) > 0 {
		log.Printf("changed snapshot sections: %v", change.Sections)
	}
}
return statusStream.Wait()
```

`snapshot.Collect` can return a non-empty snapshot and an error when only some HTTP sections fail. Inspect `Snapshot.Failures()` and `Snapshot.Complete()` before deciding whether the initial state is usable.

`snapshot.Store` does not start, stop, or reconnect a stream. The caller owns stream lifecycle.

## Local and remote differences

| Behavior | Local WebSocket | Remote MQTT |
| --- | --- | --- |
| Create | `client.NewStatusStream()` | `remoteClient.NewStatusStream()` |
| Immediate snapshot request | Supported | Returns `stream.ErrSnapshotUnsupported` |
| Transport ownership | Stream owns its WebSocket | Caller owns the MQTT transport |
| Reconnection | Uses stream reconnect options | Uses stream reconnect options and renews the firmware lease |
| Message representation | Text and binary messages | Binary messages |

Read [Remote MQTT](../integrations/remote-mqtt.md) for connection and shutdown ownership.
