package remote_test

import (
	"context"
	"errors"

	"github.com/lxdb/busylib-go/remote"
)

type exampleTransport struct{}

func (exampleTransport) Publish(context.Context, remote.Message) error {
	return errors.New("example transport is not connected")
}

func (exampleTransport) Subscribe(context.Context, remote.SubscriptionRequest) (remote.Subscription, error) {
	return nil, errors.New("example transport is not connected")
}

func ExampleNewClient() {
	client, err := remote.NewClient(exampleTransport{}, "firmware-session", remote.WithClientID("example"))
	if err != nil {
		return
	}
	defer func() { _ = client.Close() }()
	_ = client.Device()
}
