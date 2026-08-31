// Package remote adapts a caller-supplied MQTT 5 transport to the BUSY Bar
// firmware's remote HTTP and status-stream protocols.
//
// The package deliberately does not choose a broker, MQTT client, credentials,
// or authorization policy. Those concerns belong to the caller's Transport.
// The github.com/lxdb/busylib-go/pahotransport module provides an optional
// Eclipse Paho MQTT 5 adapter.
//
// A Client owns its MQTT-backed HTTP subscriptions and optional status stream,
// but it does not close the caller's Transport. Close the Client before the
// Transport.
package remote
