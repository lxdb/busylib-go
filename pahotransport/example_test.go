package pahotransport_test

import (
	"context"
	"errors"
	"log"
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
		log.Print(err)
		return
	}
	transport, err := pahotransport.Dial(ctx, autopaho.ClientConfig{
		ServerUrls: []*url.URL{broker},
		ClientConfig: paho.ClientConfig{
			ClientID: "busylib-example",
		},
	})
	if err != nil {
		log.Print(err)
		return
	}

	client, err := remote.NewClient(transport, "firmware-session", remote.WithClientID("example"))
	if err != nil {
		log.Print(errors.Join(err, transport.Close()))
		return
	}
	defer func() {
		clientErr := client.Close()
		transportErr := transport.Close()
		if err := errors.Join(clientErr, transportErr); err != nil {
			log.Print(err)
		}
	}()
	_ = client.Device()
}
