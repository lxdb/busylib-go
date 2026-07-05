package busylib

import (
	"encoding/json"
	"errors"
	"net/http"
)

type ResponseMode string

const (
	ResponseModeJSON  ResponseMode = "json"
	ResponseModeBytes ResponseMode = "bytes"
	ResponseModeText  ResponseMode = "text"
)

type Response struct {
	Method     string
	Path       string
	StatusCode int
	Header     http.Header
	RequestID  string
	Body       []byte
}

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
