package busylib

import (
	internalstatusstream "github.com/lxdb/busylib-go/internal/statusstream"
	publicstream "github.com/lxdb/busylib-go/stream"
)

// NewStatusStream creates a one-shot local status stream using this client's
// address, local access token, HTTP transport, timeout, and API-version cache. The
// caller must start and eventually stop or wait for the stream.
func (c *Client) NewStatusStream(options ...publicstream.Option) (publicstream.Stream, error) {
	if c.endpointMode != EndpointLocal {
		return nil, validationError("GET", "/api/status/ws", "local WebSocket status streaming is unavailable in remote mode", nil)
	}
	baseURL := *c.baseURL
	return internalstatusstream.New(internalstatusstream.Config{
		BaseURL:            &baseURL,
		HTTPClient:         c.httpClient,
		Timeout:            c.timeout,
		LocalAccessToken:   c.localAccessToken,
		VersionNegotiation: c.versionNegotiation != VersionNegotiationDisabled,
		APISemVer:          c.APISemVer,
		RefreshAPISemVer:   c.RefreshAPISemVer,
	}, options...)
}
