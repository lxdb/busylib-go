// Package snapshot collects and retains best-effort BUSY Bar state.
//
// Collect reads independent HTTP endpoints and records field-local failures.
// Store starts from such a snapshot and merges typed status-stream updates. It
// copies retained state and does not own a client, stream, or background task.
package snapshot
