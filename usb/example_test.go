package usb_test

import (
	"context"
	"log"

	"github.com/lxdb/busylib-go/usb"
)

func ExampleClient_Commands() {
	client, err := usb.NewClient(usb.WithAddress("10.0.4.20:23"))
	if err != nil {
		log.Print(err)
		return
	}
	response, err := client.Commands().Uptime(context.Background())
	if err != nil {
		log.Print(err)
		return
	}
	log.Print(response.Output)
}
