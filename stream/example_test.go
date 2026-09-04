package stream_test

import (
	"context"
	"errors"

	"github.com/lxdb/busylib-go/stream"
)

func ExampleStream() {
	consume := func(ctx context.Context, statusStream stream.Stream) error {
		if err := statusStream.Start(ctx); err != nil {
			return err
		}

		statuses := statusStream.Statuses()
		for {
			select {
			case status, ok := <-statuses:
				if !ok {
					return statusStream.Wait()
				}
				_ = status
			case <-ctx.Done():
				return errors.Join(ctx.Err(), statusStream.Stop())
			}
		}
	}

	_ = consume
}
