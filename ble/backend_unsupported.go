//go:build !darwin || !cgo

package ble

import (
	"context"
	"time"
)

type unsupportedBackend struct{}

func (unsupportedBackend) Scan(context.Context, time.Duration) ([]Peripheral, error) {
	return nil, ErrUnsupported
}

func (unsupportedBackend) Connect(context.Context, Identifier, time.Duration) (connection, error) {
	return nil, ErrUnsupported
}

func newPlatformBackend() backend { return unsupportedBackend{} }
