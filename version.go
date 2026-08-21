package busylib

import (
	"context"
	"errors"
)

type versionInfo struct {
	APISemVer string `json:"api_semver"`
}

type versionRefresh struct {
	done  chan struct{}
	value string
	err   error
}

// APISemVer returns the device API semantic version.
// It caches a successful response and coalesces concurrent first requests.
func (c *Client) APISemVer(ctx context.Context) (string, error) {
	c.versionMu.Lock()
	if c.apiSemVer != "" {
		apiSemVer := c.apiSemVer
		c.versionMu.Unlock()
		return apiSemVer, nil
	}
	if c.versionInFlight != nil {
		refresh := c.versionInFlight
		c.versionMu.Unlock()
		select {
		case <-refresh.done:
			if refresh.err != nil {
				return "", refresh.err
			}
			return refresh.value, nil
		case <-ctx.Done():
			return "", versionError("GET", "/api/version", "", "", ctx.Err())
		}
	}

	refresh := &versionRefresh{done: make(chan struct{})}
	c.versionInFlight = refresh
	c.versionMu.Unlock()

	value, err := c.fetchAPISemVer(ctx)

	c.versionMu.Lock()
	if err == nil {
		c.apiSemVer = value
	}
	refresh.value = value
	refresh.err = err
	c.versionInFlight = nil
	close(refresh.done)
	c.versionMu.Unlock()

	return value, err
}

// RefreshAPISemVer fetches the current device API semantic version and replaces
// the cached value after a successful response.
func (c *Client) RefreshAPISemVer(ctx context.Context) (string, error) {
	value, err := c.fetchAPISemVer(ctx)
	if err != nil {
		return "", err
	}

	c.versionMu.Lock()
	c.apiSemVer = value
	c.versionMu.Unlock()
	return value, nil
}

func (c *Client) fetchAPISemVer(ctx context.Context) (string, error) {
	response, err := c.Do(ctx, Request{
		Method:       "GET",
		Path:         "/api/version",
		ResponseMode: ResponseModeJSON,
	})
	if err != nil {
		return "", versionError("GET", "/api/version", "", "", err)
	}

	var payload versionInfo
	if err := response.DecodeJSON(&payload); err != nil {
		return "", versionError("GET", "/api/version", response.RequestID, "", err)
	}
	if payload.APISemVer == "" {
		return "", versionError(
			"GET",
			"/api/version",
			response.RequestID,
			"device API version response did not include api_semver",
			errors.New("missing api_semver"),
		)
	}

	return payload.APISemVer, nil
}
