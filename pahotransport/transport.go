package pahotransport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/lxdb/busylib-go/remote"
)

const disconnectTimeout = 10 * time.Second

const subscriptionBuffer = 16

const (
	reconcileRetryInitial = 25 * time.Millisecond
	reconcileRetryMaximum = time.Second
)

var (
	// ErrSlowConsumer reports that one local subscription could not keep up
	// with incoming messages and was terminated without blocking Paho.
	ErrSlowConsumer = errors.New("paho subscription consumer is too slow")
)

type connectionManager interface {
	AwaitConnection(context.Context) error
	Disconnect(context.Context) error
	Publish(context.Context, *paho.Publish) (*paho.PublishResponse, error)
	Subscribe(context.Context, *paho.Subscribe) (*paho.Suback, error)
	Unsubscribe(context.Context, *paho.Unsubscribe) (*paho.Unsuback, error)
}

var newConnection = func(ctx context.Context, config autopaho.ClientConfig) (connectionManager, error) {
	return autopaho.NewConnection(ctx, config)
}

// Transport owns one reconnecting Eclipse Paho MQTT 5 connection.
type Transport struct {
	manager  connectionManager
	lifetime context.Context
	cancel   context.CancelFunc
	done     <-chan struct{}

	mu            sync.RWMutex
	subscriptions map[string]map[*subscription]struct{}
	operations    chan brokerOperation
	reconcile     chan struct{}
	reconnect     chan struct{}
	workers       sync.WaitGroup
	closeOnce     sync.Once
	closeErr      error
}

var _ remote.Transport = (*Transport)(nil)

// Dial creates, connects, and owns a reconnecting Eclipse Paho transport.
// The context bounds the initial connection only. Close ends the connection's
// lifetime and must be called after all remote clients that use the transport.
func Dial(ctx context.Context, config autopaho.ClientConfig) (*Transport, error) {
	lifetime, cancel := context.WithCancel(context.Background())
	transport := &Transport{
		lifetime:      lifetime,
		cancel:        cancel,
		done:          lifetime.Done(),
		subscriptions: make(map[string]map[*subscription]struct{}),
		operations:    make(chan brokerOperation),
		reconcile:     make(chan struct{}, 1),
		reconnect:     make(chan struct{}, 1),
	}

	onConnectionUp := config.OnConnectionUp
	config.OnConnectionUp = func(manager *autopaho.ConnectionManager, ack *paho.Connack) {
		transport.requestReconnect()
		if onConnectionUp != nil {
			onConnectionUp(manager, ack)
		}
	}
	config.OnPublishReceived = append(
		append([]func(paho.PublishReceived) (bool, error){}, config.OnPublishReceived...),
		transport.route,
	)

	manager, err := newConnection(lifetime, config)
	if err != nil {
		cancel()
		return nil, err
	}
	transport.manager = manager
	transport.workers.Add(1)
	go transport.reconcileLoop()
	if err := manager.AwaitConnection(ctx); err != nil {
		return nil, errors.Join(err, transport.Close())
	}
	return transport, nil
}

// Close terminates subscriptions and disconnects the owned Paho manager.
func (t *Transport) Close() error {
	if t == nil || t.manager == nil || t.lifetime == nil || t.cancel == nil || t.done == nil {
		return remote.ErrClosed
	}
	t.closeOnce.Do(func() {
		t.failAll(remote.ErrClosed)
		t.cancel()
		t.workers.Wait()
		ctx, cancel := context.WithTimeout(context.Background(), disconnectTimeout)
		defer cancel()
		t.closeErr = t.manager.Disconnect(ctx)
	})
	return t.closeErr
}

func (t *Transport) requestReconcile() {
	if t == nil {
		return
	}
	select {
	case <-t.done:
		return
	default:
	}
	select {
	case t.reconcile <- struct{}{}:
	default:
	}
}

func (t *Transport) requestReconnect() {
	if t == nil {
		return
	}
	select {
	case <-t.done:
		return
	default:
	}
	select {
	case t.reconnect <- struct{}{}:
	default:
	}
}

func (t *Transport) reconcileLoop() {
	defer t.workers.Done()
	broker := make(map[string]remote.QoS)
	retryDelay := reconcileRetryInitial
	var retryTimer *time.Timer
	var retry <-chan time.Time
	clearRetry := func() {
		if retryTimer != nil {
			retryTimer.Stop()
			retryTimer = nil
			retry = nil
		}
		retryDelay = reconcileRetryInitial
	}
	scheduleRetry := func() {
		if retryTimer != nil {
			return
		}
		retryTimer = time.NewTimer(retryDelay)
		retry = retryTimer.C
		retryDelay = min(retryDelay*2, reconcileRetryMaximum)
	}
	handleResult := func(err error) {
		if err == nil || errors.Is(err, autopaho.ConnectionDownError) {
			clearRetry()
			return
		}
		var failure *brokerError
		if errors.As(err, &failure) && failure.operation == "subscribe" {
			clearRetry()
			t.failTopic(failure.topic, failure.err)
			return
		}
		scheduleRetry()
	}
	defer clearRetry()
	for {
		select {
		case <-t.done:
			return
		case operation := <-t.operations:
			err := t.reconcileBroker(operation.ctx, broker)
			handleResult(err)
			operation.result <- err
		case <-t.reconnect:
			clearRetry()
			clear(broker)
			ctx, cancel := context.WithTimeout(context.Background(), disconnectTimeout)
			err := t.reconcileBroker(ctx, broker)
			cancel()
			handleResult(err)
		case <-t.reconcile:
			ctx, cancel := context.WithTimeout(context.Background(), disconnectTimeout)
			err := t.reconcileBroker(ctx, broker)
			cancel()
			handleResult(err)
		case <-retry:
			retryTimer = nil
			retry = nil
			ctx, cancel := context.WithTimeout(context.Background(), disconnectTimeout)
			err := t.reconcileBroker(ctx, broker)
			cancel()
			handleResult(err)
		}
	}
}

func (t *Transport) route(received paho.PublishReceived) (bool, error) {
	packet := received.Packet
	if packet == nil {
		return false, nil
	}
	t.mu.RLock()
	registered := t.subscriptions[packet.Topic]
	subscriptions := make([]*subscription, 0, len(registered))
	for item := range registered {
		subscriptions = append(subscriptions, item)
	}
	t.mu.RUnlock()
	if len(subscriptions) == 0 {
		return false, nil
	}
	for _, item := range subscriptions {
		if int64(len(packet.Payload)) > item.maximum {
			item.fail(remote.ErrMessageTooLarge)
			t.remove(item)
			continue
		}
		message := remote.Message{
			Topic:   packet.Topic,
			Payload: append([]byte(nil), packet.Payload...),
			QoS:     remote.QoS(packet.QoS),
		}
		if packet.Properties != nil {
			message.Properties.ResponseTopic = packet.Properties.ResponseTopic
			message.Properties.CorrelationData = append([]byte(nil), packet.Properties.CorrelationData...)
			message.Properties.MessageExpiryIntervalSeconds = cloneUint32(packet.Properties.MessageExpiry)
		}
		if !item.deliver(message) {
			item.fail(ErrSlowConsumer)
			t.remove(item)
		}
	}
	return true, nil
}

// Publish sends one remote protocol message.
func (t *Transport) Publish(ctx context.Context, message remote.Message) error {
	if t == nil || t.manager == nil || t.isClosed() {
		return remote.ErrClosed
	}
	if err := validateExactTopic(message.Topic); err != nil {
		return err
	}
	if message.QoS > remote.QoSExactlyOnce {
		return errors.New("paho publish QoS must be 0, 1, or 2")
	}
	ctx, cancel := t.withLifetime(ctx)
	defer cancel()
	_, err := t.manager.Publish(ctx, &paho.Publish{
		Topic:   message.Topic,
		Payload: append([]byte(nil), message.Payload...),
		QoS:     byte(message.QoS),
		Properties: &paho.PublishProperties{
			ResponseTopic:   message.Properties.ResponseTopic,
			CorrelationData: append([]byte(nil), message.Properties.CorrelationData...),
			MessageExpiry:   cloneUint32(message.Properties.MessageExpiryIntervalSeconds),
		},
	})
	return err
}

// Subscribe creates one exact-topic subscription.
func (t *Transport) Subscribe(ctx context.Context, request remote.SubscriptionRequest) (remote.Subscription, error) {
	if t == nil || t.manager == nil || t.isClosed() {
		return nil, remote.ErrClosed
	}
	if err := validateExactTopic(request.Topic); err != nil {
		return nil, err
	}
	if request.QoS > remote.QoSExactlyOnce {
		return nil, errors.New("paho subscription QoS must be 0, 1, or 2")
	}
	if request.MaxPayloadBytes <= 0 {
		return nil, errors.New("paho subscription maximum payload must be greater than zero")
	}
	item := &subscription{
		transport: t,
		topic:     request.Topic,
		qos:       request.QoS,
		maximum:   request.MaxPayloadBytes,
		messages:  make(chan remote.Message, subscriptionBuffer),
		done:      make(chan struct{}),
	}
	t.mu.Lock()
	if t.isClosed() {
		t.mu.Unlock()
		return nil, remote.ErrClosed
	}
	if t.subscriptions[request.Topic] == nil {
		t.subscriptions[request.Topic] = make(map[*subscription]struct{})
	}
	t.subscriptions[request.Topic][item] = struct{}{}
	t.mu.Unlock()
	if err := t.reconcileSync(ctx); err != nil {
		t.remove(item)
		item.fail(err)
		return nil, err
	}
	return item, nil
}

func validateExactTopic(topic string) error {
	if topic == "" || strings.ContainsAny(topic, "+#\x00") {
		return errors.New("paho topic must be one exact MQTT topic")
	}
	return nil
}

func (t *Transport) isClosed() bool {
	if t.done == nil {
		return true
	}
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}

func cloneUint32(value *uint32) *uint32 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

type brokerOperation struct {
	ctx    context.Context
	result chan error
}

func (t *Transport) reconcileSync(ctx context.Context) error {
	operation := brokerOperation{ctx: ctx, result: make(chan error, 1)}
	select {
	case t.operations <- operation:
	case <-t.done:
		return remote.ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-operation.result:
		return err
	case <-t.done:
		return remote.ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *Transport) reconcileBroker(ctx context.Context, broker map[string]remote.QoS) error {
	ctx, cancel := t.withLifetime(ctx)
	defer cancel()

	desired := make(map[string]remote.QoS)
	t.mu.RLock()
	for topic, subscriptions := range t.subscriptions {
		for item := range subscriptions {
			if current, exists := desired[topic]; !exists || item.qos > current {
				desired[topic] = item.qos
			}
		}
	}
	t.mu.RUnlock()
	for topic, qos := range desired {
		if current, exists := broker[topic]; exists && current == qos {
			continue
		}
		ack, err := t.manager.Subscribe(ctx, &paho.Subscribe{Subscriptions: []paho.SubscribeOptions{{Topic: topic, QoS: byte(qos)}}})
		if err != nil {
			return &brokerError{operation: "subscribe", topic: topic, err: err}
		}
		if err := validateSuback(topic, ack); err != nil {
			return &brokerError{operation: "subscribe", topic: topic, err: err}
		}
		broker[topic] = qos
	}
	for topic := range broker {
		if _, exists := desired[topic]; exists {
			continue
		}
		ack, err := t.manager.Unsubscribe(ctx, &paho.Unsubscribe{Topics: []string{topic}})
		if err != nil {
			return &brokerError{operation: "unsubscribe", topic: topic, err: err}
		}
		if err := validateUnsuback(topic, ack); err != nil {
			return &brokerError{operation: "unsubscribe", topic: topic, err: err}
		}
		delete(broker, topic)
	}
	return nil
}

func (t *Transport) withLifetime(ctx context.Context) (context.Context, context.CancelFunc) {
	merged, cancel := context.WithCancel(ctx)
	stopLifetimeCancellation := context.AfterFunc(t.lifetime, cancel)
	return merged, func() {
		stopLifetimeCancellation()
		cancel()
	}
}

type brokerError struct {
	operation string
	topic     string
	err       error
}

func (e *brokerError) Error() string {
	return fmt.Sprintf("%s %q: %v", e.operation, e.topic, e.err)
}

func (e *brokerError) Unwrap() error { return e.err }

func validateSuback(topic string, ack *paho.Suback) error {
	if ack == nil || len(ack.Reasons) != 1 {
		return fmt.Errorf("subscribe to %q returned an invalid acknowledgment", topic)
	}
	if ack.Reasons[0] >= 0x80 {
		return fmt.Errorf("subscribe to %q was rejected with MQTT reason 0x%02x", topic, ack.Reasons[0])
	}
	return nil
}

func validateUnsuback(topic string, ack *paho.Unsuback) error {
	if ack == nil || len(ack.Reasons) != 1 {
		return fmt.Errorf("unsubscribe from %q returned an invalid acknowledgment", topic)
	}
	if ack.Reasons[0] >= 0x80 && ack.Reasons[0] != 0x11 {
		return fmt.Errorf("unsubscribe from %q was rejected with MQTT reason 0x%02x", topic, ack.Reasons[0])
	}
	return nil
}

func (t *Transport) remove(item *subscription) {
	t.mu.Lock()
	registered := t.subscriptions[item.topic]
	delete(registered, item)
	if len(registered) == 0 {
		delete(t.subscriptions, item.topic)
	}
	t.mu.Unlock()
	t.requestReconcile()
}

func (t *Transport) failAll(err error) {
	t.mu.Lock()
	var subscriptions []*subscription
	for _, registered := range t.subscriptions {
		for item := range registered {
			subscriptions = append(subscriptions, item)
		}
	}
	clear(t.subscriptions)
	t.mu.Unlock()
	for _, item := range subscriptions {
		item.fail(err)
	}
}

func (t *Transport) failTopic(topic string, err error) {
	t.mu.Lock()
	registered := t.subscriptions[topic]
	delete(t.subscriptions, topic)
	t.mu.Unlock()
	for item := range registered {
		item.fail(err)
	}
	t.requestReconcile()
}

type subscription struct {
	transport *Transport
	topic     string
	qos       remote.QoS
	maximum   int64
	messages  chan remote.Message
	done      chan struct{}

	mu        sync.Mutex
	err       error
	failOnce  sync.Once
	closeOnce sync.Once
	closeErr  error
}

func (s *subscription) Receive(ctx context.Context) (remote.Message, error) {
	select {
	case <-s.done:
		return remote.Message{}, s.terminalError()
	default:
	}
	select {
	case message := <-s.messages:
		return message, nil
	case <-s.done:
		return remote.Message{}, s.terminalError()
	case <-ctx.Done():
		return remote.Message{}, ctx.Err()
	}
}

func (s *subscription) Close() error {
	s.closeOnce.Do(func() {
		s.fail(remote.ErrClosed)
		s.transport.remove(s)
		ctx, cancel := context.WithTimeout(context.Background(), disconnectTimeout)
		defer cancel()
		s.closeErr = s.transport.reconcileSync(ctx)
	})
	return s.closeErr
}

func (s *subscription) deliver(message remote.Message) bool {
	select {
	case <-s.done:
		return true
	default:
	}
	select {
	case s.messages <- message:
		return true
	case <-s.done:
		return true
	default:
		return false
	}
}

func (s *subscription) fail(err error) {
	s.failOnce.Do(func() {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		close(s.done)
	})
}

func (s *subscription) terminalError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		return remote.ErrClosed
	}
	return s.err
}
