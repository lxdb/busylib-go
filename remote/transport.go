package remote

import "context"

// QoS is an MQTT quality-of-service level.
type QoS byte

const (
	// QoSAtMostOnce requests delivery without acknowledgment or retry.
	QoSAtMostOnce QoS = 0
	// QoSAtLeastOnce requests acknowledged delivery with possible duplicates.
	QoSAtLeastOnce QoS = 1
	// QoSExactlyOnce requests acknowledged delivery without duplicates.
	QoSExactlyOnce QoS = 2
)

// Properties contains the MQTT 5 properties used by the firmware protocols.
// Slice and pointer values follow the ownership rules of the enclosing Message.
type Properties struct {
	ResponseTopic                string
	CorrelationData              []byte
	MessageExpiryIntervalSeconds *uint32
}

// Message is one MQTT application message. A publisher must consume or copy
// its slices before Publish returns. A subscription transfers ownership of
// received slices to the caller.
type Message struct {
	Topic      string
	Payload    []byte
	QoS        QoS
	Properties Properties
}

// SubscriptionRequest describes one exact-topic MQTT subscription.
// MaxPayloadBytes must be positive. A transport must reject a larger payload
// before retaining or copying it for the subscriber.
type SubscriptionRequest struct {
	Topic           string
	QoS             QoS
	MaxPayloadBytes int64
}

// Transport is the caller-owned MQTT 5 connection used by a remote Client.
// Client.Close does not close it. Publish must consume or copy the message and
// its slices before returning. Implementations must support concurrent calls.
type Transport interface {
	// Publish sends one MQTT application message. It must consume or copy all
	// message data before returning and support concurrent calls.
	Publish(context.Context, Message) error
	// Subscribe creates an exact-topic subscription. The implementation must
	// validate MaxPayloadBytes and enforce it before retaining or copying each
	// delivered payload for the subscriber. If a payload exceeds the limit, the
	// implementation must stop delivery and make Receive return an error that is
	// or wraps ErrMessageTooLarge.
	Subscribe(context.Context, SubscriptionRequest) (Subscription, error)
}

// Subscription receives messages until Close is called or Receive fails.
// A received Message and its slices remain valid and owned by the caller.
// Close must be idempotent, safe to call concurrently with Receive, and
// unblock an outstanding Receive call.
type Subscription interface {
	// Receive waits for the next message, context cancellation, Close, or a
	// terminal subscription failure. The caller owns a returned Message.
	Receive(context.Context) (Message, error)
	// Close releases the subscription and unblocks Receive. It must be safe for
	// concurrent and repeated calls.
	Close() error
}
