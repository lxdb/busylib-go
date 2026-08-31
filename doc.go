// Package busylib provides a concurrent-safe Go client for BUSY Bar devices.
// NewClient configures one local or remote HTTP endpoint. Its service groups
// expose device features, and NewStatusStream creates a one-shot local status
// stream. Callers own operation contexts and must handle returned errors.
package busylib
