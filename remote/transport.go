package remote

import "context"

// QoS is an MQTT quality-of-service level.
type QoS byte

const (
	QoSAtMostOnce  QoS = 0
	QoSAtLeastOnce QoS = 1
	QoSExactlyOnce QoS = 2
)

// Properties contains the MQTT 5 properties used by the firmware protocols.
type Properties struct {
	ResponseTopic                string
	CorrelationData              []byte
	MessageExpiryIntervalSeconds *uint32
}

// Message is one MQTT application message.
type Message struct {
	Topic      string
	Payload    []byte
	QoS        QoS
	Properties Properties
}

// SubscriptionRequest describes one exact-topic MQTT subscription.
type SubscriptionRequest struct {
	Topic string
	QoS   QoS
}

// Transport is the caller-owned MQTT 5 connection used by a remote Client.
// Client.Close does not close it. Publish must consume or copy the message and
// its slices before returning. Implementations must support concurrent calls.
type Transport interface {
	Publish(context.Context, Message) error
	Subscribe(context.Context, SubscriptionRequest) (Subscription, error)
}

// Subscription receives messages until Close is called or Receive fails.
// Close must be safe to call concurrently with Receive and unblock an
// outstanding Receive call.
type Subscription interface {
	Receive(context.Context) (Message, error)
	Close() error
}
