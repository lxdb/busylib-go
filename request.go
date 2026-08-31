package busylib

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	internalapi "github.com/lxdb/busylib-go/internal/api"
)

// Request describes one BUSY Bar API operation before validation and normalization.
type Request struct {
	Method       string
	Path         string
	Query        url.Values
	Header       http.Header
	Body         Body
	ResponseMode ResponseMode
	RequestID    string
	SessionID    string
}

// PreparedRequest is an immutable normalized request returned by Prepare.
// It is safe to execute more than once only when the request body is repeatable.
type PreparedRequest struct {
	method       string
	path         string
	targetURL    url.URL
	header       http.Header
	responseMode ResponseMode
	requestID    string
	body         *preparedBody
}

// Method returns the normalized HTTP method.
func (p *PreparedRequest) Method() string { return p.method }

// Path returns the canonical API path.
func (p *PreparedRequest) Path() string { return p.path }

// URL returns a copy of the normalized target URL.
func (p *PreparedRequest) URL() url.URL { return p.targetURL }

// Header returns a copy of the prepared HTTP headers.
func (p *PreparedRequest) Header() http.Header { return p.header.Clone() }

// ResponseMode returns the validated response mode.
func (p *PreparedRequest) ResponseMode() ResponseMode { return p.responseMode }

// RequestID returns the request correlation identifier.
func (p *PreparedRequest) RequestID() string { return p.requestID }

// Prepare validates a request and creates a reusable transport-ready value.
// Each DoPrepared call receives its own copy of mutable request state.
func (c *Client) Prepare(request Request) (*PreparedRequest, error) {
	method := strings.ToUpper(strings.TrimSpace(request.Method))
	if method == "" {
		return nil, validationError("", request.Path, "request method must not be empty", nil)
	}

	path, err := canonicalAPIPath(request.Path)
	if err != nil {
		return nil, validationError(method, request.Path, "", err)
	}
	if c.endpointMode == EndpointRemote && internalapi.IsRemoteBlockedOperation(method, path) {
		return nil, validationError(method, path, method+" "+path+" is blocked by the firmware MQTT remote transport", nil)
	}

	body := emptyPreparedBody()
	if request.Body != nil {
		body, err = request.Body.prepareBody()
		if err != nil {
			return nil, validationError(method, path, "prepare request body: "+err.Error(), err)
		}
	}

	header := cloneHeader(request.Header)
	if err := validateCallerAuthHeaders(header); err != nil {
		return nil, validationError(method, path, "", err)
	}
	responseMode, err := validateResponseMode(request.ResponseMode)
	if err != nil {
		return nil, validationError(method, path, "", err)
	}

	requestID := headerValue(header, headerRequestID)
	if requestID == "" {
		requestID = request.RequestID
	}
	if requestID == "" {
		requestID = c.requestIDGenerator()
	}
	header.Set(headerRequestID, requestID)

	sessionID := request.SessionID
	if sessionID == "" {
		sessionID = c.sessionID
	}
	if sessionID != "" {
		header.Set(headerSessionID, sessionID)
	}

	if c.endpointMode == EndpointLocal && c.localAccessKey != "" {
		header.Set(headerAPIToken, c.localAccessKey)
	}

	if body.contentType != "" && headerValue(header, "Content-Type") == "" {
		header.Set("Content-Type", body.contentType)
	}

	targetURL := *c.baseURL
	targetURL.Path = path
	if len(request.Query) > 0 {
		targetURL.RawQuery = request.Query.Encode()
	}

	return &PreparedRequest{
		method:       method,
		path:         path,
		targetURL:    targetURL,
		header:       header,
		responseMode: responseMode,
		requestID:    requestID,
		body:         body,
	}, nil
}

// Do validates, prepares, and executes one API request.
// The response body is limited by the client's configured maximum size.
func (c *Client) Do(ctx context.Context, request Request) (*Response, error) {
	if ctx == nil {
		return nil, validationError(request.Method, request.Path, "context must not be nil", nil)
	}
	prepared, err := c.Prepare(request)
	if err != nil {
		return nil, err
	}
	return c.DoPrepared(ctx, prepared)
}

func (c *Client) doStreamTo(ctx context.Context, request Request, writer io.Writer) (*Response, int64, error) {
	if ctx == nil {
		return nil, 0, validationError(request.Method, request.Path, "context must not be nil", nil)
	}
	if writer == nil {
		return nil, 0, validationError(request.Method, request.Path, "writer must not be nil", nil)
	}
	prepared, err := c.Prepare(request)
	if err != nil {
		return nil, 0, err
	}
	return c.doPreparedTo(ctx, prepared, writer)
}

// DoPrepared executes a request created by Prepare.
// A prepared request can be executed multiple times only when its body is
// repeatable.
func (c *Client) DoPrepared(ctx context.Context, prepared *PreparedRequest) (*Response, error) {
	ctx, cancel, execution, err := c.preparedExecution(ctx, prepared)
	if cancel != nil {
		defer cancel()
	}
	if err != nil {
		return nil, err
	}

	return c.executePrepared(ctx, execution)
}

func (c *Client) doPreparedTo(ctx context.Context, prepared *PreparedRequest, writer io.Writer) (*Response, int64, error) {
	ctx, cancel, execution, err := c.preparedExecution(ctx, prepared)
	if cancel != nil {
		defer cancel()
	}
	if err != nil {
		return nil, 0, err
	}

	return c.executePreparedTo(ctx, execution, writer)
}

func (c *Client) preparedExecution(ctx context.Context, prepared *PreparedRequest) (context.Context, context.CancelFunc, *executionRequest, error) {
	if ctx == nil {
		return ctx, nil, nil, validationError("", "", "context must not be nil", nil)
	}
	if prepared == nil {
		return ctx, nil, nil, validationError("", "", "prepared request must not be nil", nil)
	}
	if prepared.body == nil || prepared.targetURL.Scheme == "" || prepared.targetURL.Host == "" {
		return ctx, nil, nil, validationError(prepared.method, prepared.path, "prepared request is incomplete", nil)
	}
	if _, err := validateResponseMode(prepared.responseMode); err != nil {
		return ctx, nil, nil, validationError(prepared.method, prepared.path, "", err)
	}
	var cancel context.CancelFunc
	if c.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
	}

	execution := prepared.executionCopy()
	if execution.header == nil {
		execution.header = make(http.Header)
	}
	execution.maxResponseBytes = c.maxResponseBytes

	if c.shouldNegotiateVersion(execution) {
		apiSemVer, err := c.APISemVer(ctx)
		if err != nil {
			if cancel != nil {
				cancel()
			}
			return ctx, nil, nil, err
		}
		execution.header.Set(headerAPISemVer, apiSemVer)
	}

	return ctx, cancel, execution, nil
}

type executionRequest struct {
	method           string
	path             string
	url              *url.URL
	header           http.Header
	responseMode     ResponseMode
	requestID        string
	body             *preparedBody
	maxResponseBytes int64
}

func (p *PreparedRequest) executionCopy() *executionRequest {
	targetURL := p.targetURL
	return &executionRequest{
		method:           p.method,
		path:             p.path,
		url:              &targetURL,
		header:           p.header.Clone(),
		responseMode:     p.responseMode,
		requestID:        p.requestID,
		body:             p.body,
		maxResponseBytes: 0,
	}
}

func (c *Client) executePrepared(ctx context.Context, execution *executionRequest) (*Response, error) {
	result, err := c.executePreparedWith(ctx, execution, readBufferedResponse)
	if err != nil {
		return nil, err
	}
	return result.response, nil
}

func (c *Client) executePreparedTo(ctx context.Context, execution *executionRequest, writer io.Writer) (*Response, int64, error) {
	result, err := c.executePreparedWith(ctx, execution, streamResponseTo(writer))
	if err != nil {
		if result != nil {
			return nil, result.written, err
		}
		return nil, 0, err
	}
	return result.response, result.written, nil
}

type executionResult struct {
	response *Response
	written  int64
}

type responseHandler func(*http.Response, *executionRequest, int) (*executionResult, error)

func (c *Client) executePreparedWith(ctx context.Context, execution *executionRequest, handleResponse responseHandler) (*executionResult, error) {
	maxAttempts := c.retryPolicy.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if !execution.body.repeatable {
		maxAttempts = 1
	}

	transportAttempts := 0
	compatibilityRetried := false
	for {
		transportAttempts++
		response, err := c.sendOnce(ctx, execution)
		if err != nil {
			if transportAttempts < maxAttempts {
				sleep(ctx, c.retryPolicy.Backoff)
				continue
			}
			return nil, requestError(execution, execution.requestID, transportAttempts, err)
		}

		if c.canRetryCompatibility(execution, response.StatusCode, compatibilityRetried) {
			if _, err := readAndCloseResponseBody(execution, response, transportAttempts); err != nil {
				return nil, err
			}
			if !execution.body.repeatable {
				return nil, versionError(
					execution.method,
					execution.path,
					responseRequestID(response, execution.requestID),
					"cannot retry API version negotiation with a non-repeatable request body",
					nil,
				)
			}
			compatibilityRetried = true
			apiSemVer, err := c.RefreshAPISemVer(ctx)
			if err != nil {
				return nil, err
			}
			execution.header.Set(headerAPISemVer, apiSemVer)
			continue
		}

		if response.StatusCode >= 400 {
			body, err := readAndCloseResponseBody(execution, response, transportAttempts)
			if err != nil {
				return nil, err
			}
			return nil, newAPIError(
				execution.method,
				execution.path,
				execution.requestID,
				response.StatusCode,
				response.Header.Get(headerRequestID),
				body,
			)
		}

		return handleResponse(response, execution, transportAttempts)
	}
}

func readBufferedResponse(response *http.Response, execution *executionRequest, attempts int) (*executionResult, error) {
	body, err := readAndCloseResponseBody(execution, response, attempts)
	if err != nil {
		return nil, err
	}
	if execution.responseMode == ResponseModeJSON && !json.Valid(body) {
		return nil, wrapProtocolError(
			execution.method,
			execution.path,
			responseRequestID(response, execution.requestID),
			body,
			errors.New("expected JSON response body"),
		)
	}
	return &executionResult{response: responseSummary(response, execution, body)}, nil
}

func streamResponseTo(writer io.Writer) responseHandler {
	return func(response *http.Response, execution *executionRequest, attempts int) (*executionResult, error) {
		requestID := responseRequestID(response, execution.requestID)
		n, copyErr := io.Copy(writer, response.Body)
		closeErr := response.Body.Close()
		result := &executionResult{written: n}
		if copyErr != nil {
			return result, requestError(execution, requestID, attempts, copyErr)
		}
		if closeErr != nil {
			return result, requestError(execution, requestID, attempts, closeErr)
		}
		result.response = responseSummary(response, execution, nil)
		return result, nil
	}
}

func readAndCloseResponseBody(execution *executionRequest, response *http.Response, attempts int) ([]byte, error) {
	requestID := responseRequestID(response, execution.requestID)
	reader := io.Reader(response.Body)
	if execution.maxResponseBytes < math.MaxInt64 {
		reader = io.LimitReader(response.Body, execution.maxResponseBytes+1)
	}
	body, readErr := io.ReadAll(reader)
	closeErr := response.Body.Close()
	if readErr != nil {
		return nil, requestError(execution, requestID, attempts, readErr)
	}
	if int64(len(body)) > execution.maxResponseBytes {
		return nil, requestError(execution, requestID, attempts, ErrResponseTooLarge)
	}
	if closeErr != nil {
		return nil, requestError(execution, requestID, attempts, closeErr)
	}
	return body, nil
}

func responseSummary(response *http.Response, execution *executionRequest, body []byte) *Response {
	return &Response{
		Method:     execution.method,
		Path:       execution.path,
		StatusCode: response.StatusCode,
		Header:     response.Header.Clone(),
		RequestID:  responseRequestID(response, execution.requestID),
		Body:       body,
	}
}

func requestError(execution *executionRequest, requestID string, attempts int, err error) *RequestError {
	return &RequestError{
		Method:    execution.method,
		Path:      execution.path,
		RequestID: requestID,
		Attempts:  attempts,
		Err:       err,
	}
}

func (c *Client) sendOnce(ctx context.Context, execution *executionRequest) (*http.Response, error) {
	var body io.ReadCloser
	var err error
	if execution.body != nil {
		body, err = execution.body.open()
		if err != nil {
			return nil, err
		}
	}

	httpRequest, err := http.NewRequestWithContext(ctx, execution.method, execution.url.String(), body)
	if err != nil {
		if body != nil {
			_ = body.Close()
		}
		return nil, err
	}
	httpRequest.Header = execution.header.Clone()
	if execution.body != nil && execution.body.contentLength >= 0 {
		httpRequest.ContentLength = execution.body.contentLength
	}

	response, err := c.httpClient.Do(httpRequest)
	if err != nil && body != nil {
		_ = body.Close()
	}
	return response, err
}

func (c *Client) shouldNegotiateVersion(execution *executionRequest) bool {
	return c.versionNegotiation != VersionNegotiationDisabled && execution.path != "/api/version"
}

func (c *Client) canRetryCompatibility(execution *executionRequest, statusCode int, alreadyRetried bool) bool {
	return c.versionNegotiation != VersionNegotiationDisabled &&
		execution.path != "/api/version" &&
		statusCode == http.StatusMethodNotAllowed &&
		!alreadyRetried
}

func validateCallerAuthHeaders(header http.Header) error {
	if headerValue(header, "Authorization") != "" {
		return errors.New("authorization is not supported; transport authentication must be configured outside busylib.Client")
	}
	if headerValue(header, headerAPIToken) != "" {
		return errors.New("X-API-Token must be configured with WithLocalAccessKey")
	}
	return nil
}

func canonicalAPIPath(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		return "", errors.New("request path must not be empty")
	}
	if strings.Contains(path, "://") {
		return "", errors.New("request path must be relative")
	}
	if strings.HasPrefix(path, "/busybar") {
		return "", errors.New("request path must use the /api contract path")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path == "/api" || strings.HasPrefix(path, "/api/") {
		return path, nil
	}
	return "/api" + path, nil
}

func cloneHeader(header http.Header) http.Header {
	if header == nil {
		return http.Header{}
	}
	return header.Clone()
}

func responseRequestID(response *http.Response, fallback string) string {
	if response == nil {
		return fallback
	}
	if requestID := headerValue(response.Header, headerRequestID); requestID != "" {
		return requestID
	}
	return fallback
}

func headerValue(header http.Header, name string) string {
	if value := header.Get(name); value != "" {
		return value
	}
	lowerName := strings.ToLower(name)
	for key, values := range header {
		if strings.ToLower(key) == lowerName && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func sleep(ctx context.Context, duration time.Duration) {
	if duration <= 0 {
		return
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
