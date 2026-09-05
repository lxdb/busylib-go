// Package ble connects to a BUSY Bar over Bluetooth Low Energy.
//
// The CoreBluetooth backend requires macOS and CGO. Other builds compile, but
// Scan and Connect return ErrUnsupported.
//
// Scan returns opaque CoreBluetooth identifiers. Connect retrieves exactly the
// selected known peripheral and does not scan, enable device BLE, request pair
// mode, or remove a saved pairing. macOS presents any required Bluetooth
// permission and pairing prompts.
//
// Device returns the existing root *busylib.Client. Its services construct
// normal HTTP requests, and the BLE transport serializes those requests over
// the Nordic UART Service without DNS or TCP. NewStatusStream receives FFE1
// notifications through the shared stream.Stream contract; it does not use the
// HTTP transport.
//
// Client owns the CoreBluetooth connection and its notifications. The BLE
// module is independently versioned so the root module remains free of
// CoreBluetooth and CGO dependencies.
package ble
