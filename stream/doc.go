// Package stream defines the one-shot status stream contract shared by local
// WebSocket and remote MQTT clients. Start opens a stream once. Call Stop when
// the caller no longer needs it, consume Messages or Statuses as required, and
// call Wait to observe the stable terminal or cleanup error.
package stream
