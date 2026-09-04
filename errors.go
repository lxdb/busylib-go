package busylib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const maxErrorExcerpt = 256

// ErrResponseTooLarge reports a response that exceeds the configured buffer
// limit. Match it with errors.Is through the returned RequestError.
var ErrResponseTooLarge = errors.New("response exceeds the configured buffer limit")

// APIError reports a non-success response returned by the device API. Payload
// preserves a decoded JSON error object when available. Excerpt is bounded and
// whitespace-normalized but can still contain sensitive device data.
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	RequestID  string
	// DeviceCode contains the firmware error_code value when present. It falls
	// back to the legacy code value and represents either value as a string.
	DeviceCode  string
	DeviceError string
	Excerpt     string
	Payload     map[string]any
}

// Error returns the request context and the device-provided failure message.
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

// RequestError reports a transport or response-body failure after all permitted
// attempts. Unwrap exposes the underlying cause to errors.Is and errors.As.
type RequestError struct {
	Method    string
	Path      string
	RequestID string
	Attempts  int
	Err       error
}

// Error returns the request context, attempt count, and transport cause.
func (e *RequestError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("%s %s request failed after %d attempt(s): %v (request_id=%s)", e.Method, e.Path, e.Attempts, e.Err, e.RequestID)
	}
	return fmt.Sprintf("%s %s request failed after %d attempt(s): %v", e.Method, e.Path, e.Attempts, e.Err)
}

// Unwrap returns the transport cause.
func (e *RequestError) Unwrap() error {
	return e.Err
}

// ProtocolError reports a response that does not match the expected format.
// Excerpt is bounded but can still contain sensitive device data.
type ProtocolError struct {
	Method    string
	Path      string
	RequestID string
	Excerpt   string
	Err       error
}

// Error returns the request context and payload failure.
func (e *ProtocolError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("%s %s returned an invalid payload", e.Method, e.Path)
	}
	return fmt.Sprintf("%s %s returned an invalid payload: %v", e.Method, e.Path, e.Err)
}

// Unwrap returns the payload decoding cause.
func (e *ProtocolError) Unwrap() error {
	return e.Err
}

// VersionError reports a failed API version negotiation or compatibility retry.
type VersionError struct {
	Method    string
	Path      string
	RequestID string
	Message   string
	Err       error
}

// Error returns the version negotiation failure message.
func (e *VersionError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return fmt.Sprintf("API version negotiation failed: %v", e.Err)
	}
	return "API version negotiation failed"
}

// Unwrap returns the version discovery or compatibility cause.
func (e *VersionError) Unwrap() error {
	return e.Err
}

// ValidationError reports caller input that was rejected before transport use.
type ValidationError struct {
	Method  string
	Path    string
	Message string
	Err     error
}

// Error returns the actionable validation message.
func (e *ValidationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "invalid BUSY Bar request"
}

// Unwrap returns the underlying validation cause when one exists.
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
		if code, ok := payload["error_code"]; ok {
			apiError.DeviceCode = fmt.Sprint(code)
		} else if code, ok := payload["code"]; ok {
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
