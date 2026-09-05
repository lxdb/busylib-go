package busylib

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
)

// ResponseMode selects how Client buffers and validates a response body. The
// zero value is ResponseModeJSON.
type ResponseMode string

const (
	// ResponseModeJSON requires a valid JSON response body.
	ResponseModeJSON ResponseMode = "json"
	// ResponseModeBytes accepts any response body as bytes.
	ResponseModeBytes ResponseMode = "bytes"
	// ResponseModeText accepts any response body without validating UTF-8.
	ResponseModeText ResponseMode = "text"
)

// Response contains one buffered device response and its request context.
// Header and Body are owned by the Response and can be modified by the caller.
type Response struct {
	Method     string
	Path       string
	StatusCode int
	Header     http.Header
	RequestID  string
	Body       []byte
}

// DecodeJSON decodes Body into target. It returns ProtocolError with request
// context and a bounded payload excerpt when decoding fails.
func (r *Response) DecodeJSON(target any) error {
	if err := json.Unmarshal(r.Body, target); err != nil {
		return &ProtocolError{
			Method:    r.Method,
			Path:      r.Path,
			RequestID: r.RequestID,
			Excerpt:   excerpt(r.Body),
			Err:       err,
		}
	}
	return nil
}

func normalizeResponseMode(mode ResponseMode) ResponseMode {
	if mode == "" {
		return ResponseModeJSON
	}
	return mode
}

func validateResponseMode(mode ResponseMode) (ResponseMode, error) {
	mode = normalizeResponseMode(mode)
	switch mode {
	case ResponseModeJSON, ResponseModeBytes, ResponseModeText:
		return mode, nil
	default:
		return "", errors.New("response mode must be json, bytes, or text")
	}
}

type successResponse struct {
	Result string `json:"result"`
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body Body, target any) error {
	response, err := c.Do(ctx, Request{
		Method:       method,
		Path:         path,
		Query:        query,
		Body:         body,
		ResponseMode: ResponseModeJSON,
	})
	if err != nil {
		return err
	}
	if target == nil {
		return nil
	}
	return response.DecodeJSON(target)
}

func (c *Client) doSuccess(ctx context.Context, method, path string, query url.Values, body Body) error {
	var out successResponse
	return c.doJSON(ctx, method, path, query, body, &out)
}

func (c *Client) doBytes(ctx context.Context, method, path string, query url.Values, body Body) ([]byte, error) {
	response, err := c.Do(ctx, Request{
		Method:       method,
		Path:         path,
		Query:        query,
		Body:         body,
		ResponseMode: ResponseModeBytes,
	})
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}
