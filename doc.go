// Package busylib provides an HTTP client for BUSY Bar devices that is safe for
// concurrent use.
//
// NewClient uses the device's USB-network endpoint by default. A Client groups
// the device API into services such as System, Display, Storage, and Update.
// Use Do or Prepare only when no typed service exposes the required operation.
//
// NewStatusStream creates a one-shot local WebSocket stream. Package remote
// provides the corresponding MQTT client. Callers own operation contexts and
// must handle the errors returned by requests, streams, and cleanup.
package busylib
