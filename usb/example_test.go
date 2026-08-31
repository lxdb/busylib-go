package usb_test

import (
	"context"
	"log"
	"time"

	"github.com/lxdb/busylib-go/usb"
)

func ExampleClient_Commands() {
	client, err := usb.NewClient(usb.WithAddress("10.0.4.20:23"))
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := client.Commands().Uptime(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Print(response.Output)
}
