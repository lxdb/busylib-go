package pahotransport_test

import (
	"context"
	"net/url"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/lxdb/busylib-go/pahotransport"
	"github.com/lxdb/busylib-go/remote"
)

func ExampleDial() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	broker, err := url.Parse("mqtt://broker.example:1883")
	if err != nil {
		return
	}
	transport, err := pahotransport.Dial(ctx, autopaho.ClientConfig{
		ServerUrls: []*url.URL{broker},
		ClientConfig: paho.ClientConfig{
			ClientID: "busylib-example",
		},
	})
	if err != nil {
		return
	}
	defer func() { _ = transport.Close() }()

	client, err := remote.NewClient(transport, "firmware-session", remote.WithClientID("example"))
	if err != nil {
		return
	}
	defer func() { _ = client.Close() }()
	_ = client.Device()
}
