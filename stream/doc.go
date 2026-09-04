// Package stream defines the one-shot status-stream contract shared by local
// WebSocket and remote MQTT clients.
//
// Start opens a stream once. Consume Messages or Statuses until the selected
// channel closes. Call Stop when the stream is no longer needed, and use Wait
// to observe its stable terminal or cleanup error. A finished stream cannot be
// restarted.
package stream
