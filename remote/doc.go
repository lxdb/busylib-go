// Package remote adapts a caller-supplied MQTT 5 transport to the BUSY Bar
// firmware's remote HTTP and status-stream protocols.
//
// The package deliberately does not choose a broker, MQTT client, credentials,
// or authorization policy. Those concerns belong to the caller's Transport.
package remote
