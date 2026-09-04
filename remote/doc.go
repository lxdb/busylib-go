// Package remote adapts a caller-supplied MQTT 5 transport to the BUSY Bar
// firmware's remote request and status-stream protocols.
//
// The package deliberately does not choose a broker, MQTT client, credentials,
// or authorization policy. Those concerns belong to the caller's Transport.
// The github.com/lxdb/busylib-go/pahotransport module provides an optional
// Eclipse Paho MQTT 5 adapter.
//
// A Client owns its request subscriptions and optional status stream. It does
// not close the caller's Transport. Close all clients before closing their
// shared transport, and report errors from both cleanup steps.
package remote
