// Package pahotransport provides an optional Eclipse Paho MQTT 5 transport for
// github.com/lxdb/busylib-go/remote.
//
// Dial connects and returns a Transport that owns its Paho connection manager.
// The transport restores active broker subscriptions after reconnection and
// multiplexes local subscribers that share a topic. The context passed to Dial
// bounds only the initial connection. Close ends the connection lifetime and
// must run after all remote clients that use the transport have closed.
package pahotransport
