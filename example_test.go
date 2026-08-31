package busylib_test

import (
	"context"
	"log"
	"time"

	busylib "github.com/lxdb/busylib-go"
)

func ExampleClient() {
	client, err := busylib.NewClient(busylib.WithBaseURL("http://10.0.4.20"))
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	status, err := client.System().Status(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Print(status.Firmware.Version)
}

func ExampleNewDisplayElements() {
	elements := busylib.NewDisplayElements(
		"example",
		busylib.NewTextElement("title", "Build complete", busylib.FontNormal),
	)
	_ = elements
}
