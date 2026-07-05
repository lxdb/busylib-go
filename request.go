package busylib

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	internalapi "github.com/lxdb/busylib-go/internal/api"
)

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

// PreparedRequest is a normalized request description returned by Prepare.
// It is safe to execute more than once only when the request body is repeatable.
// DoPrepared never mutates the exported request fields.
type PreparedRequest struct {
	Method       string
	Path         string
	URL          *url.URL
	Header       http.Header
	ResponseMode ResponseMode
	RequestID    string

	body *preparedBody
}

func (c *Client) Prepare(_ context.Context, request Request) (*PreparedRequest, error) {
	method := strings.ToUpper(strings.TrimSpace(request.Method))
	if method == "" {
		return nil, validationError("", request.Path, "request method must not be empty", nil)
	}

	path, err := canonicalAPIPath(request.Path)
	if err != nil {
		return nil, validationError(method, request.Path, "", err)
	}
	if c.endpointMode == EndpointProxy && internalapi.IsLocalOnlyOperation(method, path) {
		return nil, validationError(method, path, method+" "+path+" is local-only and cannot be sent in proxy mode", nil)
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

	if c.endpointMode == EndpointProxy {
		header.Set(headerBearer, "Bearer "+c.cloudBearerToken)
	} else if c.localAccessKey != "" {
		header.Set(headerAPIToken, c.localAccessKey)
	}

	if body.contentType != "" && headerValue(header, "Content-Type") == "" {
		header.Set("Content-Type", body.contentType)
	}

	targetURL := *c.baseURL
	targetURL.Path = transportPath(c.endpointMode, path)
	if len(request.Query) > 0 {
		targetURL.RawQuery = request.Query.Encode()
	}

	return &PreparedRequest{
		Method:       method,
		Path:         path,
		URL:          &targetURL,
		Header:       header,
		ResponseMode: responseMode,
		RequestID:    requestID,
		body:         body,
	}, nil
}

func (c *Client) Do(ctx context.Context, request Request) (*Response, error) {
	prepared, err := c.Prepare(ctx, request)
	if err != nil {
		return nil, err
	}
	return c.DoPrepared(ctx, prepared)
}

// DoPrepared executes a request created by Prepare.
// It does not mutate the exported PreparedRequest fields. A prepared request
// can be executed multiple times only when its body is repeatable.
func (c *Client) DoPrepared(ctx context.Context, prepared *PreparedRequest) (*Response, error) {
	if prepared == nil {
		return nil, validationError("", "", "prepared request must not be nil", nil)
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	execution := prepared.executionCopy()

	if c.shouldNegotiateVersion(execution) {
		apiSemVer, err := c.APISemVer(ctx)
		if err != nil {
			return nil, err
		}
		execution.header.Set(headerAPISemVer, apiSemVer)
	}

	return c.executePrepared(ctx, execution)
}

type executionRequest struct {
	method       string
	path         string
	url          *url.URL
	header       http.Header
	responseMode ResponseMode
	requestID    string
	body         *preparedBody
}

func (p *PreparedRequest) executionCopy() *executionRequest {
	targetURL := *p.URL
	return &executionRequest{
		method:       p.Method,
		path:         p.Path,
		url:          &targetURL,
		header:       p.Header.Clone(),
		responseMode: p.ResponseMode,
		requestID:    p.RequestID,
		body:         p.body,
	}
}

func (c *Client) executePrepared(ctx context.Context, execution *executionRequest) (*Response, error) {
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
			return nil, &RequestError{
				Method:    execution.method,
				Path:      execution.path,
				RequestID: execution.requestID,
				Attempts:  transportAttempts,
				Err:       err,
			}
		}

		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil {
			return nil, &RequestError{
				Method:    execution.method,
				Path:      execution.path,
				RequestID: responseRequestID(response, execution.requestID),
				Attempts:  transportAttempts,
				Err:       readErr,
			}
		}
		if closeErr != nil {
			return nil, &RequestError{
				Method:    execution.method,
				Path:      execution.path,
				RequestID: responseRequestID(response, execution.requestID),
				Attempts:  transportAttempts,
				Err:       closeErr,
			}
		}

		if c.canRetryCompatibility(execution, response.StatusCode, compatibilityRetried) {
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
			return nil, newAPIError(
				execution.method,
				execution.path,
				execution.requestID,
				response.StatusCode,
				response.Header.Get(headerRequestID),
				body,
			)
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

		return &Response{
			Method:     execution.method,
			Path:       execution.path,
			StatusCode: response.StatusCode,
			Header:     response.Header.Clone(),
			RequestID:  responseRequestID(response, execution.requestID),
			Body:       body,
		}, nil
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
	if headerValue(header, headerBearer) != "" {
		return errors.New("Authorization must be configured with WithCloudBearerToken")
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

func transportPath(mode EndpointMode, canonicalPath string) string {
	if mode == EndpointProxy {
		suffix := strings.TrimPrefix(canonicalPath, "/api")
		if suffix == "" {
			suffix = "/"
		}
		return "/busybar" + suffix
	}
	return canonicalPath
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
