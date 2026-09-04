package ble

import (
	"context"
	"time"
)

// The firmware accepts at most 237 bytes in one NUS write, even when
// CoreBluetooth reports a larger maximum write length.
const firmwareWriteLimit = 237

type backend interface {
	Scan(context.Context, time.Duration) ([]Peripheral, error)
	Connect(context.Context, Identifier, time.Duration) (connection, error)
}

type connection interface {
	fragmentWriter
	MaximumWriteValueLength() int
	SetHTTPNotificationHandler(func([]byte))
	SetHTTPErrorHandler(func(error))
	SetStateErrorHandler(func(error))
	SetDisconnectHandler(func(error))
	EnableStateNotifications(context.Context, func([]byte)) error
	DisableStateNotifications(context.Context) error
	Reconnect(context.Context, time.Duration) error
	Close() error
}
