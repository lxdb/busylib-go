package pahotransport

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/lxdb/busylib-go/remote"
)

func TestBrokerReconnectRestoresSubscription(t *testing.T) {
	address := os.Getenv("BUSYLIB_MQTT_BROKER_URL")
	if address == "" {
		t.Skip("BUSYLIB_MQTT_BROKER_URL is not set")
	}
	broker, err := url.Parse(address)
	if err != nil {
		t.Fatalf("parse broker URL: %v", err)
	}
	connectionUp := make(chan struct{}, 4)
	clientID := fmt.Sprintf("busylib-integration-%d", time.Now().UnixNano())
	dialCtx, cancelDial := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDial()
	transport, err := Dial(dialCtx, autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{broker},
		CleanStartOnInitialConnection: true,
		ReconnectBackoff:              autopaho.NewConstantBackoff(50 * time.Millisecond),
		ConnectTimeout:                time.Second,
		OnConnectionUp: func(*autopaho.ConnectionManager, *paho.Connack) {
			select {
			case connectionUp <- struct{}{}:
			default:
			}
		},
		ClientConfig: paho.ClientConfig{ClientID: clientID},
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = transport.Close() }()
	select {
	case <-connectionUp:
	case <-time.After(time.Second):
		t.Fatal("initial connection callback was not observed")
	}
	topic := "busylib/integration/" + clientID
	subscription, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
		Topic:           topic,
		QoS:             remote.QoSAtLeastOnce,
		MaxPayloadBytes: 1024,
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer func() { _ = subscription.Close() }()
	assertBrokerDelivery(t, transport, subscription, topic, "before")

	manager, ok := transport.manager.(interface{ TerminateConnectionForTest() })
	if !ok {
		t.Fatal("Paho manager does not expose its connection-loss test hook")
	}
	manager.TerminateConnectionForTest()
	select {
	case <-connectionUp:
	case <-time.After(5 * time.Second):
		t.Fatal("Paho did not reconnect")
	}
	assertBrokerDelivery(t, transport, subscription, topic, "after")
}

func assertBrokerDelivery(t *testing.T, transport *Transport, subscription remote.Subscription, topic, payload string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for attempt := 0; ; attempt++ {
		publishCtx, cancelPublish := context.WithTimeout(context.Background(), time.Second)
		err := transport.Publish(publishCtx, remote.Message{
			Topic:   topic,
			Payload: []byte(payload),
			QoS:     remote.QoSAtLeastOnce,
		})
		cancelPublish()
		if err == nil {
			receiveCtx, cancelReceive := context.WithTimeout(context.Background(), 200*time.Millisecond)
			message, receiveErr := subscription.Receive(receiveCtx)
			cancelReceive()
			if receiveErr == nil && string(message.Payload) == payload {
				return
			}
		}
		select {
		case <-deadline.C:
			t.Fatalf("payload %q was not delivered after %d attempts", payload, attempt+1)
		default:
		}
	}
}
