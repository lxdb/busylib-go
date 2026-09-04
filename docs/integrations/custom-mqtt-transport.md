# Implement a custom MQTT transport

Implement `remote.Transport` only when the application cannot use the optional Paho adapter. The interface is small, but its ownership, concurrency, and payload limits are part of the public contract.

```go
type Transport interface {
    Publish(context.Context, Message) error
    Subscribe(context.Context, SubscriptionRequest) (Subscription, error)
}

type Subscription interface {
    Receive(context.Context) (Message, error)
    Close() error
}
```

## Publication contract

`Publish` must support concurrent calls. It must consume or copy `Message` and all contained slices before it returns. The caller can reuse or change those slices after the call.

The implementation must preserve the MQTT 5 response topic, correlation data, message expiry interval, topic, payload, and requested QoS. These fields carry the remote HTTP request and response protocol.

## Subscription contract

`Subscribe` creates an exact-topic subscription. It must reject an empty or unsupported topic according to the transport's own validation and must reject a non-positive `SubscriptionRequest.MaxPayloadBytes` value.

For each delivered message, the implementation must:

1. Check the payload length before retaining or copying the payload for that subscriber.
2. Stop or reject delivery when the payload exceeds the requested limit.
3. Make `Receive` return an error that is or wraps `remote.ErrMessageTooLarge`.
4. Return a message whose data remains valid and is owned by the caller.

Do not receive an unbounded payload into a subscriber queue and check its size later. The limit protects memory at the delivery boundary.

## Close and concurrency contract

`Transport` implementations must support concurrent `Publish` and `Subscribe` calls. A `Subscription` must support a concurrent `Receive` and `Close`.

`Subscription.Close` must be idempotent and must unblock an outstanding `Receive`. `Receive` must also return when its context ends. Do not leave broker callbacks blocked on a slow local consumer; use a bounded queue or another bounded delivery policy.

The caller owns the transport connection. Closing a `remote.Client` closes that client's subscriptions, not the transport. The transport can be shared by several remote clients and must remain open until all of them are closed.

## Verify an implementation

Test the observable contract, including:

- exact-topic subscription and MQTT 5 property preservation;
- concurrent publication and subscription;
- non-positive and oversized payload rejection before retention;
- `errors.Is(err, remote.ErrMessageTooLarge)` for oversized delivery;
- context cancellation of `Receive`;
- concurrent, repeated `Close` calls that unblock `Receive`;
- connection loss, reconnection, and subscription restoration when the implementation promises reconnection; and
- a slow subscriber that does not block the broker receive path.

The [Paho adapter](../../pahotransport/README.md) is a concrete implementation of these rules and can serve as a behavioral reference. Its buffering and reconnection strategy are adapter details, not additional requirements on every transport.
