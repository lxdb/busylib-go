//go:build darwin && cgo

package ble

func newPlatformBackend() backend { return coreBluetoothBackend{} }
