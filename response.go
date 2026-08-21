package busylib

import (
	"encoding/json"
	"errors"
	"net/http"
)

// ResponseMode selects how Client buffers and validates a response body.
type ResponseMode string

const (
	// ResponseModeJSON requires a valid JSON response body.
	ResponseModeJSON ResponseMode = "json"
	// ResponseModeBytes accepts any response body as bytes.
	ResponseModeBytes ResponseMode = "bytes"
	// ResponseModeText accepts any response body as text bytes.
	ResponseModeText ResponseMode = "text"
)

// Response contains one buffered device response and its request context.
type Response struct {
	Method     string
	Path       string
	StatusCode int
	Header     http.Header
	RequestID  string
	Body       []byte
}

// DecodeJSON decodes Body into target and returns ProtocolError on failure.
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
