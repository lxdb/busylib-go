package ble_test

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/lxdb/busylib-go/ble"
)

func Example() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	peripherals, err := ble.Scan(ctx, 10*time.Second)
	if err != nil {
		log.Print(err)
		return
	}
	client, err := ble.Connect(ctx, peripherals[0].Identifier)
	if err != nil {
		log.Print(err)
		return
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Print(err)
		}
	}()

	status, err := client.Device().System().Status(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Print(status.Firmware.Version)
}

func ExampleClient_NewStatusStream() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	peripherals, err := ble.Scan(ctx, 10*time.Second)
	if err != nil {
		log.Print(err)
		return
	}
	client, err := ble.Connect(ctx, peripherals[0].Identifier)
	if err != nil {
		log.Print(err)
		return
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Print(err)
		}
	}()

	statusStream, err := client.NewStatusStream()
	if err != nil {
		log.Print(err)
		return
	}
	if err := statusStream.Start(ctx); err != nil {
		log.Print(err)
		return
	}
	messages := statusStream.Messages()
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				if err := statusStream.Wait(); err != nil {
					log.Print(err)
				}
				return
			}
			if message.DecodeError != nil {
				log.Print(message.DecodeError)
				continue
			}
			log.Print(message.Updates)

		case <-ctx.Done():
			if err := errors.Join(ctx.Err(), statusStream.Stop()); err != nil {
				log.Print(err)
			}
			return
		}
	}
}
