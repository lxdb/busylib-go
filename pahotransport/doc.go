// Package pahotransport provides an optional Eclipse Paho MQTT 5 transport for
// github.com/lxdb/busylib-go/remote.
//
// A Transport owns its connection manager, reconnects active broker
// subscriptions, and multiplexes local subscriptions that share a topic. Dial
// bounds the initial connection with the supplied context; Close ends the
// connection lifetime.
package pahotransport
