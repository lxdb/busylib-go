// Package usb accesses the raw CLI exposed by the BUSY Bar USB-network
// interface.
//
// Client opens a connection for each operation. Session keeps one serialized
// connection for several commands. Continuous commands run until the prompt
// returns or their context is canceled. This package is independent of the
// core HTTP client.
package usb
