package busylib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const maxErrorExcerpt = 256

type APIError struct {
	Method      string
	Path        string
	StatusCode  int
	RequestID   string
	DeviceCode  string
	DeviceError string
	Excerpt     string
	Payload     map[string]any
}

func (e *APIError) Error() string {
	message := e.DeviceError
	if message == "" {
		message = fmt.Sprintf("HTTP %d", e.StatusCode)
	}
	if e.RequestID != "" {
		return fmt.Sprintf("%s %s failed: %s (status=%d request_id=%s)", e.Method, e.Path, message, e.StatusCode, e.RequestID)
	}
	return fmt.Sprintf("%s %s failed: %s (status=%d)", e.Method, e.Path, message, e.StatusCode)
}

type RequestError struct {
	Method    string
	Path      string
	RequestID string
	Attempts  int
	Err       error
}

func (e *RequestError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("%s %s request failed after %d attempt(s): %v (request_id=%s)", e.Method, e.Path, e.Attempts, e.Err, e.RequestID)
	}
	return fmt.Sprintf("%s %s request failed after %d attempt(s): %v", e.Method, e.Path, e.Attempts, e.Err)
}

func (e *RequestError) Unwrap() error {
	return e.Err
}

type ProtocolError struct {
	Method    string
	Path      string
	RequestID string
	Excerpt   string
	Err       error
}

func (e *ProtocolError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("%s %s returned an invalid payload", e.Method, e.Path)
	}
	return fmt.Sprintf("%s %s returned an invalid payload: %v", e.Method, e.Path, e.Err)
}

func (e *ProtocolError) Unwrap() error {
	return e.Err
}

type VersionError struct {
	Method    string
	Path      string
	RequestID string
	Message   string
	Err       error
}

func (e *VersionError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return fmt.Sprintf("API version negotiation failed: %v", e.Err)
	}
	return "API version negotiation failed"
}

func (e *VersionError) Unwrap() error {
	return e.Err
}

type ValidationError struct {
	Method  string
	Path    string
	Message string
	Err     error
}

func (e *ValidationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "invalid BUSY Bar request"
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

func newAPIError(method, path, requestID string, statusCode int, headerRequestID string, body []byte) *APIError {
	if headerRequestID != "" {
		requestID = headerRequestID
	}
	apiError := &APIError{
		Method:     method,
		Path:       path,
		StatusCode: statusCode,
		RequestID:  requestID,
		Excerpt:    excerpt(body),
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		apiError.Payload = payload
		if value, ok := payload["error"].(string); ok {
			apiError.DeviceError = value
		} else if value, ok := payload["message"].(string); ok {
			apiError.DeviceError = value
		}
		if code, ok := payload["code"]; ok {
			apiError.DeviceCode = fmt.Sprint(code)
		}
	}
	return apiError
}

func excerpt(body []byte) string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return ""
	}
	normalized := strings.Join(strings.Fields(string(body)), " ")
	if len(normalized) <= maxErrorExcerpt {
		return normalized
	}
	return normalized[:maxErrorExcerpt] + "..."
}

func validationError(method, path, message string, err error) *ValidationError {
	if message == "" && err != nil {
		message = err.Error()
	}
	if message == "" {
		message = "invalid BUSY Bar request"
	}
	return &ValidationError{
		Method:  method,
		Path:    path,
		Message: message,
		Err:     err,
	}
}

func versionError(method, path, requestID, message string, err error) *VersionError {
	return &VersionError{
		Method:    method,
		Path:      path,
		RequestID: requestID,
		Message:   message,
		Err:       err,
	}
}

func wrapProtocolError(method, path, requestID string, body []byte, err error) *ProtocolError {
	if err == nil {
		err = errors.New("invalid payload")
	}
	return &ProtocolError{
		Method:    method,
		Path:      path,
		RequestID: requestID,
		Excerpt:   excerpt(body),
		Err:       err,
	}
}
