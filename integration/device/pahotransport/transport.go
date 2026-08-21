// Package pahotransport adapts Eclipse Paho MQTT 5 to busylib remote.Transport.
package pahotransport

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/lxdb/busylib-go/remote"
)

// Transport adapts a connected Paho connection manager.
// The caller owns the manager and its connection lifecycle.
type Transport struct {
	Manager *autopaho.ConnectionManager
}

const unsubscribeTimeout = 10 * time.Second

// Publish sends one remote protocol message.
func (t Transport) Publish(ctx context.Context, message remote.Message) error {
	if t.Manager == nil {
		return errors.New("Paho connection manager must not be nil")
	}
	properties := &paho.PublishProperties{
		ResponseTopic:   message.Properties.ResponseTopic,
		CorrelationData: append([]byte(nil), message.Properties.CorrelationData...),
		MessageExpiry:   message.Properties.MessageExpiryIntervalSeconds,
	}
	_, err := t.Manager.Publish(ctx, &paho.Publish{
		Topic:      message.Topic,
		QoS:        byte(message.QoS),
		Payload:    append([]byte(nil), message.Payload...),
		Properties: properties,
	})
	return err
}

// Subscribe creates one exact-topic subscription.
func (t Transport) Subscribe(ctx context.Context, request remote.SubscriptionRequest) (remote.Subscription, error) {
	if t.Manager == nil {
		return nil, errors.New("Paho connection manager must not be nil")
	}
	subscription := &subscription{
		manager:  t.Manager,
		topic:    request.Topic,
		messages: make(chan remote.Message, 16),
		done:     make(chan struct{}),
	}
	subscription.removeHandler = t.Manager.AddOnPublishReceived(func(received autopaho.PublishReceived) (bool, error) {
		packet := received.Packet
		if packet == nil || packet.Topic != request.Topic {
			return false, nil
		}
		message := remote.Message{
			Topic:   packet.Topic,
			Payload: append([]byte(nil), packet.Payload...),
			QoS:     remote.QoS(packet.QoS),
		}
		if packet.Properties != nil {
			message.Properties.ResponseTopic = packet.Properties.ResponseTopic
			message.Properties.CorrelationData = append([]byte(nil), packet.Properties.CorrelationData...)
			message.Properties.MessageExpiryIntervalSeconds = packet.Properties.MessageExpiry
		}
		return subscription.deliver(message)
	})

	_, err := t.Manager.Subscribe(ctx, &paho.Subscribe{Subscriptions: []paho.SubscribeOptions{{Topic: request.Topic, QoS: byte(request.QoS)}}})
	if err != nil {
		subscription.removeHandler()
		return nil, err
	}
	return subscription, nil
}

type subscription struct {
	manager interface {
		Unsubscribe(context.Context, *paho.Unsubscribe) (*paho.Unsuback, error)
	}
	topic         string
	messages      chan remote.Message
	done          chan struct{}
	removeHandler func()
	closeOnce     sync.Once
	closeErr      error
}

func (s *subscription) Receive(ctx context.Context) (remote.Message, error) {
	select {
	case <-s.done:
		return remote.Message{}, remote.ErrClosed
	default:
	}
	select {
	case message := <-s.messages:
		return message, nil
	case <-s.done:
		return remote.Message{}, remote.ErrClosed
	case <-ctx.Done():
		return remote.Message{}, ctx.Err()
	}
}

func (s *subscription) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
		if s.removeHandler != nil {
			s.removeHandler()
		}
		ctx, cancel := context.WithTimeout(context.Background(), unsubscribeTimeout)
		defer cancel()
		_, s.closeErr = s.manager.Unsubscribe(ctx, &paho.Unsubscribe{Topics: []string{s.topic}})
	})
	return s.closeErr
}

func (s *subscription) deliver(message remote.Message) (bool, error) {
	select {
	case <-s.done:
		return true, nil
	default:
	}
	select {
	case s.messages <- message:
		return true, nil
	case <-s.done:
		return true, nil
	default:
		return true, errors.New("Paho subscription consumer is too slow")
	}
}
