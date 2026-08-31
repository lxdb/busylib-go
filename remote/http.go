package remote

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync"
)

type httpResult struct {
	message Message
	err     error
}

type httpRoundTripper struct {
	transport       Transport
	requestTopic    string
	responseTopic   string
	maxMessageBytes int64

	mu           sync.Mutex
	subscription Subscription
	pending      map[string]chan httpResult
	closed       bool
	inflight     sync.WaitGroup
	closeOnce    sync.Once
	closeErr     error
}

func newHTTPRoundTripper(transport Transport, sessionID, clientID string, maxMessageBytes int64) *httpRoundTripper {
	return &httpRoundTripper{
		transport:       transport,
		requestTopic:    "sessions/" + sessionID + "/down/v1/http-request",
		responseTopic:   "sessions/" + sessionID + "/up/v1/http-response/" + clientID,
		maxMessageBytes: maxMessageBytes,
		pending:         make(map[string]chan httpResult),
	}
}

func (r *httpRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil {
		return nil, errors.New("remote HTTP request must not be nil")
	}
	if err := r.beginRoundTrip(); err != nil {
		return nil, err
	}
	defer r.inflight.Done()
	if request.Body != nil {
		defer func() { _ = request.Body.Close() }()
	}
	subscription, err := r.ensureSubscription(request.Context())
	if err != nil {
		return nil, err
	}

	correlation, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	result := make(chan httpResult, 1)
	if err := r.register(correlation, subscription, result); err != nil {
		return nil, err
	}
	registered := true
	defer func() {
		if registered {
			r.unregister(correlation, result)
		}
	}()

	payload := &messageBuffer{maximum: r.maxMessageBytes}
	if err := request.Write(payload); err != nil {
		if errors.Is(err, ErrMessageTooLarge) {
			return nil, &Error{Operation: "encode HTTP", Route: request.URL.Path, Err: ErrMessageTooLarge}
		}
		return nil, &Error{Operation: "encode HTTP", Route: request.URL.Path, Err: err}
	}
	message := Message{
		Topic:   r.requestTopic,
		Payload: payload.Bytes(),
		QoS:     QoSExactlyOnce,
		Properties: Properties{
			ResponseTopic:   r.responseTopic,
			CorrelationData: []byte(correlation),
		},
	}
	if err := r.publish(request.Context(), message); err != nil {
		return nil, &Error{Operation: "publish HTTP", Route: r.requestTopic, Err: err}
	}

	select {
	case response := <-result:
		registered = false
		if response.err != nil {
			return nil, response.err
		}
		parsed, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(response.message.Payload)), request)
		if err != nil {
			return nil, &Error{Operation: "decode HTTP", Route: r.responseTopic, Err: err}
		}
		return parsed, nil
	case <-request.Context().Done():
		return nil, request.Context().Err()
	}
}

func (r *httpRoundTripper) beginRoundTrip() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return &Error{Operation: "request HTTP", Route: r.requestTopic, Terminal: true, Err: ErrClosed}
	}
	r.inflight.Add(1)
	return nil
}

func (r *httpRoundTripper) publish(ctx context.Context, message Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrClosed
	}
	return r.transport.Publish(ctx, message)
}

func (r *httpRoundTripper) ensureSubscription(ctx context.Context) (Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, &Error{Operation: "subscribe HTTP", Route: r.responseTopic, Terminal: true, Err: ErrClosed}
	}
	if r.subscription != nil {
		return r.subscription, nil
	}
	subscription, err := r.transport.Subscribe(ctx, SubscriptionRequest{
		Topic:           r.responseTopic,
		QoS:             QoSAtLeastOnce,
		MaxPayloadBytes: r.maxMessageBytes,
	})
	if err != nil {
		return nil, &Error{Operation: "subscribe HTTP", Route: r.responseTopic, Err: err}
	}
	if subscription == nil {
		return nil, &Error{Operation: "subscribe HTTP", Route: r.responseTopic, Err: errors.New("transport returned a nil subscription")}
	}
	r.subscription = subscription
	go r.receive(subscription)
	return subscription, nil
}

func (r *httpRoundTripper) register(correlation string, subscription Subscription, result chan httpResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return &Error{Operation: "register HTTP", Route: r.responseTopic, Terminal: true, Err: ErrClosed}
	}
	if r.subscription != subscription {
		return &Error{Operation: "register HTTP", Route: r.responseTopic, Err: errors.New("response subscription changed")}
	}
	r.pending[correlation] = result
	return nil
}

func (r *httpRoundTripper) unregister(correlation string, result chan httpResult) {
	r.mu.Lock()
	if r.pending[correlation] == result {
		delete(r.pending, correlation)
	}
	r.mu.Unlock()
}

func (r *httpRoundTripper) receive(subscription Subscription) {
	for {
		message, err := subscription.Receive(context.Background())
		if err != nil {
			r.subscriptionFailed(subscription, err)
			return
		}
		if message.Topic != r.responseTopic || len(message.Properties.CorrelationData) == 0 {
			continue
		}
		correlation := string(message.Properties.CorrelationData)
		r.mu.Lock()
		result := r.pending[correlation]
		if result != nil {
			delete(r.pending, correlation)
		}
		r.mu.Unlock()
		if result != nil {
			if int64(len(message.Payload)) > r.maxMessageBytes {
				result <- httpResult{err: &Error{Operation: "receive HTTP", Route: r.responseTopic, Err: ErrMessageTooLarge}}
			} else {
				result <- httpResult{message: cloneTransportMessage(message)}
			}
		}
	}
}

type messageBuffer struct {
	buffer  bytes.Buffer
	maximum int64
}

func (b *messageBuffer) Write(data []byte) (int, error) {
	remaining := b.maximum - int64(b.buffer.Len())
	if int64(len(data)) <= remaining {
		return b.buffer.Write(data)
	}
	if remaining > 0 {
		_, _ = b.buffer.Write(data[:int(remaining)])
	}
	return int(max(remaining, 0)), ErrMessageTooLarge
}

func (b *messageBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (r *httpRoundTripper) subscriptionFailed(subscription Subscription, err error) {
	r.mu.Lock()
	if r.subscription != subscription {
		r.mu.Unlock()
		return
	}
	r.subscription = nil
	pending := r.pending
	r.pending = make(map[string]chan httpResult)
	closed := r.closed
	r.mu.Unlock()
	if closed && errors.Is(err, context.Canceled) {
		err = ErrClosed
	}
	failure := &Error{Operation: "receive HTTP", Route: r.responseTopic, Terminal: closed, Err: err}
	for _, result := range pending {
		result <- httpResult{err: failure}
	}
}

func (r *httpRoundTripper) Close() error {
	r.closeOnce.Do(func() { r.closeErr = r.close() })
	return r.closeErr
}

func (r *httpRoundTripper) close() error {
	r.mu.Lock()
	r.closed = true
	subscription := r.subscription
	r.subscription = nil
	pending := r.pending
	r.pending = make(map[string]chan httpResult)
	r.mu.Unlock()

	failure := &Error{Operation: "close HTTP", Route: r.responseTopic, Terminal: true, Err: ErrClosed}
	for _, result := range pending {
		result <- httpResult{err: failure}
	}
	var closeErr error
	if subscription != nil {
		if err := subscription.Close(); err != nil {
			closeErr = &Error{Operation: "close HTTP subscription", Route: r.responseTopic, Terminal: true, Err: err}
		}
	}
	r.inflight.Wait()
	return closeErr
}

func cloneTransportMessage(message Message) Message {
	message.Payload = append([]byte(nil), message.Payload...)
	message.Properties.CorrelationData = append([]byte(nil), message.Properties.CorrelationData...)
	return message
}
