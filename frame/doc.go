// Package frame validates and decodes BUSY Bar display captures.
//
// FromHTTP accepts the uncompressed bytes returned by the screen endpoint.
// FromProto preserves status-stream metadata and encoded data. Frame.Pixels and
// Frame.RGBA validate that metadata before they decode or convert the payload.
package frame
