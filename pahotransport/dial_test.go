package pahotransport

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/lxdb/busylib-go/remote"
)

type fakeManager struct {
	mu                sync.Mutex
	awaitErr          error
	disconnectErr     error
	publishErr        error
	published         *paho.Publish
	publishCalls      int
	publishSignal     chan struct{}
	blockPublish      bool
	subscribeErr      error
	subscribeReasons  []byte
	unsubscribeErr    error
	subscribeCalls    []*paho.Subscribe
	unsubscribeCalls  []*paho.Unsubscribe
	subscribeSignal   chan struct{}
	unsubscribeSignal chan struct{}
	blockSubscribe    bool
	blockUnsubscribe  bool
	awaitCalls        int
	disconnectCalls   int
}

func (m *fakeManager) AwaitConnection(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.awaitCalls++
	return m.awaitErr
}

func (m *fakeManager) Disconnect(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disconnectCalls++
	return m.disconnectErr
}

func (m *fakeManager) Publish(ctx context.Context, message *paho.Publish) (*paho.PublishResponse, error) {
	m.mu.Lock()
	m.published = message
	m.publishCalls++
	if m.publishSignal != nil {
		select {
		case m.publishSignal <- struct{}{}:
		default:
		}
	}
	blocked := m.blockPublish
	err := m.publishErr
	m.mu.Unlock()
	if blocked {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &paho.PublishResponse{}, err
}

func TestCloseCancelsBlockedPublish(t *testing.T) {
	manager := &fakeManager{blockPublish: true, publishSignal: make(chan struct{}, 1)}
	transport := dialWithManager(t, manager)
	publishDone := make(chan error, 1)
	go func() {
		publishDone <- transport.Publish(context.Background(), remote.Message{Topic: "device/request"})
	}()
	<-manager.publishSignal
	if err := transport.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case err := <-publishDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Publish error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked Publish did not return after Close")
	}
}

func TestPublishCopiesRemoteMessageIntoPahoPacket(t *testing.T) {
	manager := &fakeManager{}
	transport := dialWithManager(t, manager)
	defer func() { _ = transport.Close() }()

	expiry := uint32(30)
	message := remote.Message{
		Topic:   "device/request",
		Payload: []byte("payload"),
		QoS:     remote.QoSAtLeastOnce,
		Properties: remote.Properties{
			ResponseTopic:                "device/response",
			CorrelationData:              []byte("correlation"),
			MessageExpiryIntervalSeconds: &expiry,
		},
	}
	if err := transport.Publish(context.Background(), message); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	message.Payload[0] = 'X'
	message.Properties.CorrelationData[0] = 'X'
	expiry = 60

	manager.mu.Lock()
	defer manager.mu.Unlock()
	packet := manager.published
	if packet == nil || packet.Topic != "device/request" || packet.QoS != 1 || string(packet.Payload) != "payload" {
		t.Fatalf("published packet = %#v", packet)
	}
	if packet.Properties == nil || packet.Properties.ResponseTopic != "device/response" ||
		string(packet.Properties.CorrelationData) != "correlation" || packet.Properties.MessageExpiry == nil ||
		*packet.Properties.MessageExpiry != 30 {
		t.Fatalf("published properties = %#v", packet.Properties)
	}
}

func TestPublishRejectsInvalidTopicAndQoS(t *testing.T) {
	manager := &fakeManager{}
	transport := dialWithManager(t, manager)
	defer func() { _ = transport.Close() }()
	for _, message := range []remote.Message{
		{Topic: ""},
		{Topic: "device/+"},
		{Topic: "device/request", QoS: remote.QoS(3)},
	} {
		if err := transport.Publish(context.Background(), message); err == nil {
			t.Fatalf("Publish(%#v) succeeded", message)
		}
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.publishCalls != 0 {
		t.Fatalf("manager Publish calls = %d, want 0", manager.publishCalls)
	}
}

func dialWithManager(t *testing.T, manager *fakeManager) *Transport {
	t.Helper()
	originalFactory := newConnection
	newConnection = func(context.Context, autopaho.ClientConfig) (connectionManager, error) {
		return manager, nil
	}
	t.Cleanup(func() { newConnection = originalFactory })
	transport, err := Dial(context.Background(), autopaho.ClientConfig{})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	return transport
}

func (m *fakeManager) Subscribe(ctx context.Context, request *paho.Subscribe) (*paho.Suback, error) {
	m.mu.Lock()
	m.subscribeCalls = append(m.subscribeCalls, request)
	if m.subscribeSignal != nil {
		select {
		case m.subscribeSignal <- struct{}{}:
		default:
		}
	}
	reasons := m.subscribeReasons
	blocked := m.blockSubscribe
	err := m.subscribeErr
	m.mu.Unlock()
	if blocked {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if reasons == nil {
		reasons = []byte{0}
	}
	return &paho.Suback{Reasons: reasons}, err
}

func (m *fakeManager) Unsubscribe(ctx context.Context, request *paho.Unsubscribe) (*paho.Unsuback, error) {
	m.mu.Lock()
	m.unsubscribeCalls = append(m.unsubscribeCalls, request)
	if m.unsubscribeSignal != nil {
		select {
		case m.unsubscribeSignal <- struct{}{}:
		default:
		}
	}
	blocked := m.blockUnsubscribe
	err := m.unsubscribeErr
	m.mu.Unlock()
	if blocked {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &paho.Unsuback{Reasons: []byte{0}}, err
}

func TestCloseCancelsBlockedBrokerSubscribe(t *testing.T) {
	manager := &fakeManager{
		blockSubscribe:  true,
		subscribeSignal: make(chan struct{}, 1),
	}
	transport := dialWithManager(t, manager)
	subscribeDone := make(chan error, 1)
	go func() {
		_, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
			Topic:           "device/status",
			QoS:             remote.QoSAtMostOnce,
			MaxPayloadBytes: 1024,
		})
		subscribeDone <- err
	}()
	<-manager.subscribeSignal

	closeDone := make(chan error, 1)
	go func() { closeDone <- transport.Close() }()
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not cancel the blocked broker subscribe")
	}
	select {
	case err := <-subscribeDone:
		if !errors.Is(err, remote.ErrClosed) && !errors.Is(err, context.Canceled) {
			t.Fatalf("Subscribe error = %v, want closed or canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked Subscribe did not return after Close")
	}
}

func TestCloseCancelsBlockedBrokerUnsubscribe(t *testing.T) {
	manager := &fakeManager{unsubscribeSignal: make(chan struct{}, 1)}
	transport := dialWithManager(t, manager)
	subscription, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
		Topic:           "device/status",
		QoS:             remote.QoSAtMostOnce,
		MaxPayloadBytes: 1024,
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	manager.mu.Lock()
	manager.blockUnsubscribe = true
	manager.mu.Unlock()
	unsubscribeDone := make(chan error, 1)
	go func() { unsubscribeDone <- subscription.Close() }()
	<-manager.unsubscribeSignal

	closeDone := make(chan error, 1)
	go func() { closeDone <- transport.Close() }()
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not cancel the blocked broker unsubscribe")
	}
	select {
	case err := <-unsubscribeDone:
		if !errors.Is(err, remote.ErrClosed) && !errors.Is(err, context.Canceled) {
			t.Fatalf("subscription Close error = %v, want closed or canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked subscription Close did not return after transport Close")
	}
}

func TestSubscriptionsToSameTopicShareBrokerSubscription(t *testing.T) {
	manager := &fakeManager{}
	transport := dialWithManager(t, manager)
	defer func() { _ = transport.Close() }()
	request := remote.SubscriptionRequest{
		Topic:           "device/status",
		QoS:             remote.QoSAtLeastOnce,
		MaxPayloadBytes: 1024,
	}
	first, err := transport.Subscribe(context.Background(), request)
	if err != nil {
		t.Fatalf("first Subscribe: %v", err)
	}
	second, err := transport.Subscribe(context.Background(), request)
	if err != nil {
		t.Fatalf("second Subscribe: %v", err)
	}

	manager.mu.Lock()
	subscribeCalls := len(manager.subscribeCalls)
	manager.mu.Unlock()
	if subscribeCalls != 1 {
		t.Fatalf("broker subscribe calls = %d, want 1", subscribeCalls)
	}

	payload := []byte("status")
	handled, err := transport.route(paho.PublishReceived{Packet: &paho.Publish{
		Topic:   request.Topic,
		Payload: payload,
		QoS:     1,
	}})
	if err != nil || !handled {
		t.Fatalf("route = handled %t, error %v", handled, err)
	}
	payload[0] = 'X'
	firstMessage, err := first.Receive(context.Background())
	if err != nil {
		t.Fatalf("first Receive: %v", err)
	}
	secondMessage, err := second.Receive(context.Background())
	if err != nil {
		t.Fatalf("second Receive: %v", err)
	}
	if string(firstMessage.Payload) != "status" || string(secondMessage.Payload) != "status" {
		t.Fatalf("received payloads = %q, %q", firstMessage.Payload, secondMessage.Payload)
	}
	firstMessage.Payload[0] = 'Y'
	if string(secondMessage.Payload) != "status" {
		t.Fatal("subscribers share payload storage")
	}

	if err := first.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	manager.mu.Lock()
	unsubscribeCalls := len(manager.unsubscribeCalls)
	manager.mu.Unlock()
	if unsubscribeCalls != 0 {
		t.Fatalf("broker unsubscribe calls after first Close = %d, want 0", unsubscribeCalls)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	manager.mu.Lock()
	unsubscribeCalls = len(manager.unsubscribeCalls)
	manager.mu.Unlock()
	if unsubscribeCalls != 1 {
		t.Fatalf("broker unsubscribe calls after last Close = %d, want 1", unsubscribeCalls)
	}
}

func TestSlowConsumerReceivesTerminalError(t *testing.T) {
	manager := &fakeManager{}
	transport := dialWithManager(t, manager)
	defer func() { _ = transport.Close() }()
	subscription, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
		Topic:           "device/status",
		QoS:             remote.QoSAtMostOnce,
		MaxPayloadBytes: 1024,
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	for index := 0; index <= subscriptionBuffer; index++ {
		if _, err := transport.route(paho.PublishReceived{Packet: &paho.Publish{
			Topic:   "device/status",
			Payload: []byte{byte(index)},
		}}); err != nil {
			t.Fatalf("route message %d: %v", index, err)
		}
	}
	if _, err := subscription.Receive(context.Background()); !errors.Is(err, ErrSlowConsumer) {
		t.Fatalf("Receive error = %v, want ErrSlowConsumer", err)
	}
}

func TestOversizedPayloadIsRejectedBeforeDelivery(t *testing.T) {
	manager := &fakeManager{}
	transport := dialWithManager(t, manager)
	defer func() { _ = transport.Close() }()
	subscription, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
		Topic:           "device/status",
		QoS:             remote.QoSAtMostOnce,
		MaxPayloadBytes: 3,
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if _, err := transport.route(paho.PublishReceived{Packet: &paho.Publish{
		Topic:   "device/status",
		Payload: []byte("four"),
	}}); err != nil {
		t.Fatalf("route: %v", err)
	}
	if _, err := subscription.Receive(context.Background()); !errors.Is(err, remote.ErrMessageTooLarge) {
		t.Fatalf("Receive error = %v, want remote.ErrMessageTooLarge", err)
	}
}

func TestConnectionUpRestoresBrokerSubscriptions(t *testing.T) {
	manager := &fakeManager{subscribeSignal: make(chan struct{}, 2)}
	var captured autopaho.ClientConfig
	originalFactory := newConnection
	newConnection = func(_ context.Context, config autopaho.ClientConfig) (connectionManager, error) {
		captured = config
		return manager, nil
	}
	t.Cleanup(func() { newConnection = originalFactory })

	transport, err := Dial(context.Background(), autopaho.ClientConfig{})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = transport.Close() }()
	if len(captured.OnPublishReceived) != 1 {
		t.Fatalf("configured publish handlers = %d, want 1", len(captured.OnPublishReceived))
	}
	if _, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
		Topic:           "device/status",
		QoS:             remote.QoSAtLeastOnce,
		MaxPayloadBytes: 1024,
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	select {
	case <-manager.subscribeSignal:
	case <-time.After(time.Second):
		t.Fatal("initial broker subscription was not sent")
	}

	captured.OnConnectionUp(nil, &paho.Connack{})
	select {
	case <-manager.subscribeSignal:
	case <-time.After(time.Second):
		t.Fatal("broker subscription was not restored after connection up")
	}
}

func TestSubscriptionCloseUnblocksReceiveAndIsIdempotent(t *testing.T) {
	manager := &fakeManager{}
	transport := dialWithManager(t, manager)
	defer func() { _ = transport.Close() }()
	subscription, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
		Topic:           "device/status",
		QoS:             remote.QoSAtMostOnce,
		MaxPayloadBytes: 1024,
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	received := make(chan error, 1)
	go func() {
		_, err := subscription.Receive(context.Background())
		received <- err
	}()
	if err := subscription.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := subscription.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	select {
	case err := <-received:
		if !errors.Is(err, remote.ErrClosed) {
			t.Fatalf("Receive error = %v, want remote.ErrClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not unblock Receive")
	}
}

func TestBrokerSubscriptionTracksHighestRequestedQoS(t *testing.T) {
	manager := &fakeManager{}
	transport := dialWithManager(t, manager)
	defer func() { _ = transport.Close() }()
	low, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
		Topic:           "device/status",
		QoS:             remote.QoSAtMostOnce,
		MaxPayloadBytes: 1024,
	})
	if err != nil {
		t.Fatalf("low Subscribe: %v", err)
	}
	high, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
		Topic:           "device/status",
		QoS:             remote.QoSAtLeastOnce,
		MaxPayloadBytes: 1024,
	})
	if err != nil {
		t.Fatalf("high Subscribe: %v", err)
	}
	if err := high.Close(); err != nil {
		t.Fatalf("high Close: %v", err)
	}

	manager.mu.Lock()
	var qualities []byte
	for _, call := range manager.subscribeCalls {
		qualities = append(qualities, call.Subscriptions[0].QoS)
	}
	manager.mu.Unlock()
	if !reflect.DeepEqual(qualities, []byte{0, 1, 0}) {
		t.Fatalf("broker subscription QoS calls = %v, want [0 1 0]", qualities)
	}
	if err := low.Close(); err != nil {
		t.Fatalf("low Close: %v", err)
	}
}

func TestSubscribePreservesManagerErrorAndRollsBack(t *testing.T) {
	wantErr := errors.New("subscribe failed")
	manager := &fakeManager{subscribeErr: wantErr}
	transport := dialWithManager(t, manager)
	defer func() { _ = transport.Close() }()
	request := remote.SubscriptionRequest{
		Topic:           "device/status",
		QoS:             remote.QoSAtMostOnce,
		MaxPayloadBytes: 1024,
	}
	if subscription, err := transport.Subscribe(context.Background(), request); subscription != nil || !errors.Is(err, wantErr) {
		t.Fatalf("Subscribe = %#v, %v; want nil, %v", subscription, err, wantErr)
	}
	manager.mu.Lock()
	manager.subscribeErr = nil
	manager.mu.Unlock()
	subscription, err := transport.Subscribe(context.Background(), request)
	if err != nil {
		t.Fatalf("Subscribe after failure: %v", err)
	}
	if err := subscription.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestSubscribeRejectsWildcardTopic(t *testing.T) {
	manager := &fakeManager{}
	transport := dialWithManager(t, manager)
	defer func() { _ = transport.Close() }()
	for _, topic := range []string{"device/+", "device/#"} {
		subscription, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
			Topic:           topic,
			QoS:             remote.QoSAtMostOnce,
			MaxPayloadBytes: 1024,
		})
		if subscription != nil || err == nil {
			t.Fatalf("Subscribe(%q) = %#v, %v; want nil error result", topic, subscription, err)
		}
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if len(manager.subscribeCalls) != 0 {
		t.Fatalf("broker subscribe calls = %d, want 0", len(manager.subscribeCalls))
	}
}

func TestRejectedReconnectSubscriptionTerminatesSubscriber(t *testing.T) {
	manager := &fakeManager{subscribeSignal: make(chan struct{}, 2)}
	var captured autopaho.ClientConfig
	originalFactory := newConnection
	newConnection = func(_ context.Context, config autopaho.ClientConfig) (connectionManager, error) {
		captured = config
		return manager, nil
	}
	t.Cleanup(func() { newConnection = originalFactory })
	transport, err := Dial(context.Background(), autopaho.ClientConfig{})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = transport.Close() }()
	subscription, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
		Topic:           "device/status",
		QoS:             remote.QoSAtLeastOnce,
		MaxPayloadBytes: 1024,
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	<-manager.subscribeSignal
	wantErr := errors.New("subscription rejected")
	manager.mu.Lock()
	manager.subscribeErr = wantErr
	manager.mu.Unlock()
	captured.OnConnectionUp(nil, &paho.Connack{})
	select {
	case <-manager.subscribeSignal:
	case <-time.After(time.Second):
		t.Fatal("reconnect subscription was not attempted")
	}
	receiveDone := make(chan error, 1)
	go func() {
		_, err := subscription.Receive(context.Background())
		receiveDone <- err
	}()
	select {
	case err := <-receiveDone:
		if !errors.Is(err, wantErr) {
			t.Fatalf("Receive error = %v, want %v", err, wantErr)
		}
	case <-time.After(time.Second):
		t.Fatal("rejected reconnect did not terminate subscription")
	}
}

func TestRejectedReconnectTerminatesEveryRejectedTopic(t *testing.T) {
	manager := &fakeManager{subscribeSignal: make(chan struct{}, 8)}
	var captured autopaho.ClientConfig
	originalFactory := newConnection
	newConnection = func(_ context.Context, config autopaho.ClientConfig) (connectionManager, error) {
		captured = config
		return manager, nil
	}
	t.Cleanup(func() { newConnection = originalFactory })
	transport, err := Dial(context.Background(), autopaho.ClientConfig{})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = transport.Close() }()

	var subscriptions []remote.Subscription
	for _, topic := range []string{"device/status", "device/timer"} {
		subscription, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
			Topic:           topic,
			QoS:             remote.QoSAtLeastOnce,
			MaxPayloadBytes: 1024,
		})
		if err != nil {
			t.Fatalf("Subscribe(%q): %v", topic, err)
		}
		subscriptions = append(subscriptions, subscription)
		<-manager.subscribeSignal
	}

	wantErr := errors.New("subscription rejected")
	manager.mu.Lock()
	manager.subscribeErr = wantErr
	manager.mu.Unlock()
	captured.OnConnectionUp(nil, &paho.Connack{})

	for index, subscription := range subscriptions {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := subscription.Receive(ctx)
		cancel()
		if !errors.Is(err, wantErr) {
			t.Fatalf("subscription %d Receive error = %v, want %v", index, err, wantErr)
		}
	}
}

func TestFailedUnsubscribeIsRetriedUntilBrokerStateConverges(t *testing.T) {
	wantErr := errors.New("unsubscribe failed")
	manager := &fakeManager{
		unsubscribeErr:    wantErr,
		unsubscribeSignal: make(chan struct{}, 8),
	}
	transport := dialWithManager(t, manager)
	defer func() { _ = transport.Close() }()
	subscription, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
		Topic:           "device/status",
		QoS:             remote.QoSAtMostOnce,
		MaxPayloadBytes: 1024,
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := subscription.Close(); !errors.Is(err, wantErr) {
		t.Fatalf("Close error = %v, want %v", err, wantErr)
	}

	for {
		select {
		case <-manager.unsubscribeSignal:
		default:
			goto drained
		}
	}

drained:
	manager.mu.Lock()
	manager.unsubscribeErr = nil
	manager.mu.Unlock()
	select {
	case <-manager.unsubscribeSignal:
	case <-time.After(time.Second):
		t.Fatal("unsubscribe was not retried after Close returned")
	}
}

func TestConnectionDownDuringReconcileKeepsSubscriberActive(t *testing.T) {
	manager := &fakeManager{subscribeSignal: make(chan struct{}, 3)}
	var captured autopaho.ClientConfig
	originalFactory := newConnection
	newConnection = func(_ context.Context, config autopaho.ClientConfig) (connectionManager, error) {
		captured = config
		return manager, nil
	}
	t.Cleanup(func() { newConnection = originalFactory })
	transport, err := Dial(context.Background(), autopaho.ClientConfig{})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = transport.Close() }()
	subscription, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{
		Topic:           "device/status",
		QoS:             remote.QoSAtLeastOnce,
		MaxPayloadBytes: 1024,
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	<-manager.subscribeSignal

	manager.mu.Lock()
	manager.subscribeErr = autopaho.ConnectionDownError
	manager.mu.Unlock()
	captured.OnConnectionUp(nil, &paho.Connack{})
	<-manager.subscribeSignal
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := subscription.Receive(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Receive after transient reconnect error = %v, want context deadline", err)
	}

	manager.mu.Lock()
	manager.subscribeErr = nil
	manager.mu.Unlock()
	captured.OnConnectionUp(nil, &paho.Connack{})
	<-manager.subscribeSignal
	if _, err := transport.route(paho.PublishReceived{Packet: &paho.Publish{
		Topic:   "device/status",
		Payload: []byte("restored"),
	}}); err != nil {
		t.Fatalf("route after reconnect: %v", err)
	}
	message, err := subscription.Receive(context.Background())
	if err != nil || string(message.Payload) != "restored" {
		t.Fatalf("Receive after reconnect = %#v, %v", message, err)
	}
}

func TestDialOwnsConnectionLifecycleAndPreservesConnectionCallback(t *testing.T) {
	manager := &fakeManager{}
	var captured autopaho.ClientConfig
	originalFactory := newConnection
	newConnection = func(_ context.Context, config autopaho.ClientConfig) (connectionManager, error) {
		captured = config
		return manager, nil
	}
	t.Cleanup(func() { newConnection = originalFactory })

	callbackCalls := 0
	transport, err := Dial(context.Background(), autopaho.ClientConfig{
		OnConnectionUp: func(*autopaho.ConnectionManager, *paho.Connack) {
			callbackCalls++
		},
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if captured.OnConnectionUp == nil {
		t.Fatal("Dial did not install a connection callback")
	}
	captured.OnConnectionUp(nil, &paho.Connack{})
	if callbackCalls != 1 {
		t.Fatalf("original OnConnectionUp calls = %d, want 1", callbackCalls)
	}

	if err := transport.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := transport.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.awaitCalls != 1 || manager.disconnectCalls != 1 {
		t.Fatalf("lifecycle calls = await %d, disconnect %d; want 1, 1", manager.awaitCalls, manager.disconnectCalls)
	}
}

func TestDialCleansUpWhenInitialConnectionFails(t *testing.T) {
	wantErr := errors.New("await failed")
	manager := &fakeManager{awaitErr: wantErr}
	originalFactory := newConnection
	newConnection = func(context.Context, autopaho.ClientConfig) (connectionManager, error) {
		return manager, nil
	}
	t.Cleanup(func() { newConnection = originalFactory })

	transport, err := Dial(context.Background(), autopaho.ClientConfig{})
	if transport != nil || !errors.Is(err, wantErr) {
		t.Fatalf("Dial = %#v, %v; want nil, %v", transport, err, wantErr)
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.disconnectCalls != 1 {
		t.Fatalf("disconnect calls = %d, want 1", manager.disconnectCalls)
	}
}

func TestZeroTransportReturnsClosedErrors(t *testing.T) {
	var transport Transport
	if err := transport.Publish(context.Background(), remote.Message{}); !errors.Is(err, remote.ErrClosed) {
		t.Fatalf("Publish error = %v, want remote.ErrClosed", err)
	}
	if subscription, err := transport.Subscribe(context.Background(), remote.SubscriptionRequest{}); subscription != nil || !errors.Is(err, remote.ErrClosed) {
		t.Fatalf("Subscribe = %#v, %v; want nil, remote.ErrClosed", subscription, err)
	}
	if err := transport.Close(); !errors.Is(err, remote.ErrClosed) {
		t.Fatalf("Close error = %v, want remote.ErrClosed", err)
	}
}
