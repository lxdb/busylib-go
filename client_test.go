package busylib

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientDefaultsAndAddressNormalization(t *testing.T) {
	client, err := NewClient(WithRequestIDGenerator(fixedRequestID("rid-1")))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if got := client.BaseURL(); got != "http://10.0.4.20" {
		t.Fatalf("BaseURL = %q, want default local URL", got)
	}

	client, err = NewClient(
		WithBaseURL("busybar.local/ignored/path"),
		WithRequestIDGenerator(fixedRequestID("rid-1")),
	)
	if err != nil {
		t.Fatalf("NewClient with bare address: %v", err)
	}
	if got := client.BaseURL(); got != "http://busybar.local" {
		t.Fatalf("BaseURL = %q, want normalized origin", got)
	}
}

func TestPrepareAppliesLocalAuthRequestIDAndSession(t *testing.T) {
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithLocalAccessKey("1234"),
		WithSessionID("global-session"),
		WithRequestIDGenerator(fixedRequestID("rid-1")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	prepared, err := client.Prepare(context.Background(), Request{
		Method: "GET",
		Path:   "/api/status",
		Query: url.Values{
			"display": []string{"front"},
		},
		SessionID: "request-session",
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if prepared.Method != http.MethodGet {
		t.Fatalf("Method = %q", prepared.Method)
	}
	if got := prepared.URL.String(); got != "http://busybar.local/api/status?display=front" {
		t.Fatalf("URL = %q", got)
	}
	if got := prepared.Header.Get("X-API-Token"); got != "1234" {
		t.Fatalf("X-API-Token = %q", got)
	}
	if got := prepared.Header.Get("X-Request-ID"); got != "rid-1" {
		t.Fatalf("X-Request-ID = %q", got)
	}
	if got := prepared.Header.Get("x-session-id"); got != "request-session" {
		t.Fatalf("x-session-id = %q", got)
	}
	if got := prepared.RequestID; got != "rid-1" {
		t.Fatalf("RequestID = %q", got)
	}
}

func TestProxyModeRewritesPathAndSeparatesBearerAuth(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/busybar/status" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cloud-token" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-API-Token"); got != "" {
			t.Fatalf("X-API-Token = %q, want absent", got)
		}
		if got := r.Header.Get("X-API-Sem-Ver"); got != "24.3.0" {
			t.Fatalf("X-API-Sem-Ver = %q", got)
		}
		writeJSON(t, w, map[string]string{"ok": "true"})
	}))
	defer server.Close()

	client, err := NewClient(
		WithEndpointMode(EndpointProxy),
		WithBaseURL(server.URL+"/ignored"),
		WithCloudBearerToken("cloud-token"),
		WithHTTPClient(server.Client()),
		WithRequestIDGenerator(sequenceRequestID("version-rid", "request-rid")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.setCachedAPISemVerForTest("24.3.0")

	if _, err := client.Do(context.Background(), Request{
		Method:       "GET",
		Path:         "/api/status",
		ResponseMode: ResponseModeJSON,
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
}

func TestProxyModeRequiresHTTPS(t *testing.T) {
	for _, baseURL := range []string{"https://api.busy.app"} {
		if _, err := NewClient(
			WithEndpointMode(EndpointProxy),
			WithBaseURL(baseURL),
			WithCloudBearerToken("cloud-token"),
		); err != nil {
			t.Fatalf("NewClient with %s: %v", baseURL, err)
		}
	}

	for _, baseURL := range []string{"http://api.busy.app", "api.busy.app"} {
		_, err := NewClient(
			WithEndpointMode(EndpointProxy),
			WithBaseURL(baseURL),
			WithCloudBearerToken("cloud-token"),
		)
		var validationErr *ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("NewClient with %s error = %T %v, want ValidationError", baseURL, err, err)
		}
	}
}

func TestPrepareRejectsConflictingAuthHeaders(t *testing.T) {
	local, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithLocalAccessKey("1234"),
	)
	if err != nil {
		t.Fatalf("local NewClient: %v", err)
	}
	_, err = local.Prepare(context.Background(), Request{
		Method: "GET",
		Path:   "/api/status",
		Header: http.Header{
			"Authorization": []string{"Bearer cloud-token"},
		},
	})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("local conflicting auth error = %T %v, want ValidationError", err, err)
	}

	proxy, err := NewClient(
		WithEndpointMode(EndpointProxy),
		WithBaseURL("https://api.busy.app"),
		WithCloudBearerToken("cloud-token"),
	)
	if err != nil {
		t.Fatalf("proxy NewClient: %v", err)
	}
	_, err = proxy.Prepare(context.Background(), Request{
		Method: "GET",
		Path:   "/api/status",
		Header: http.Header{
			"X-API-Token": []string{"1234"},
		},
	})
	if !errors.As(err, &validationErr) {
		t.Fatalf("proxy conflicting auth error = %T %v, want ValidationError", err, err)
	}
}

func TestVersionCacheAndSemVerHeader(t *testing.T) {
	var versionCalls int
	var statusCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			versionCalls++
			if got := r.Header.Get("X-API-Sem-Ver"); got != "" {
				t.Fatalf("version request X-API-Sem-Ver = %q", got)
			}
			writeJSON(t, w, map[string]string{"api_semver": "24.3.0"})
		case "/api/status":
			statusCalls++
			if got := r.Header.Get("X-API-Sem-Ver"); got != "24.3.0" {
				t.Fatalf("status request X-API-Sem-Ver = %q", got)
			}
			writeJSON(t, w, map[string]string{"status": "ok"})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithRequestIDGenerator(sequenceRequestID("rid-v", "rid-1", "rid-2")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := client.Do(context.Background(), Request{
			Method:       "GET",
			Path:         "/api/status",
			ResponseMode: ResponseModeJSON,
		}); err != nil {
			t.Fatalf("Do %d: %v", i+1, err)
		}
	}

	if versionCalls != 1 {
		t.Fatalf("versionCalls = %d, want 1", versionCalls)
	}
	if statusCalls != 2 {
		t.Fatalf("statusCalls = %d, want 2", statusCalls)
	}
}

func TestCompatibilityRetryRefreshesVersionOnceForRepeatableBody(t *testing.T) {
	var versionCalls int
	var drawCalls int
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			versionCalls++
			writeJSON(t, w, map[string]string{"api_semver": "24.3." + string(rune('0'+versionCalls))})
		case "/api/display/draw":
			drawCalls++
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			bodies = append(bodies, string(body))
			if drawCalls == 1 {
				w.WriteHeader(http.StatusMethodNotAllowed)
				writeJSON(t, w, map[string]string{"error": "api version mismatch"})
				return
			}
			if got := r.Header.Get("X-API-Sem-Ver"); got != "24.3.2" {
				t.Fatalf("retried X-API-Sem-Ver = %q", got)
			}
			writeJSON(t, w, map[string]string{"draw": "ok"})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithRequestIDGenerator(sequenceRequestID("rid-v1", "rid-draw", "rid-v2")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := client.Do(context.Background(), Request{
		Method:       "POST",
		Path:         "/api/display/draw",
		Body:         JSONBody(map[string]string{"application_name": "app"}),
		ResponseMode: ResponseModeJSON,
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}

	if versionCalls != 2 {
		t.Fatalf("versionCalls = %d, want 2", versionCalls)
	}
	if drawCalls != 2 {
		t.Fatalf("drawCalls = %d, want 2", drawCalls)
	}
	if len(bodies) != 2 || bodies[0] != bodies[1] {
		t.Fatalf("bodies = %#v, want same JSON payload twice", bodies)
	}
}

func TestCompatibilityRetryRejectsNonRepeatableBody(t *testing.T) {
	var uploadCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			writeJSON(t, w, map[string]string{"api_semver": "24.3.0"})
		case "/api/storage/write":
			uploadCalls++
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(t, w, map[string]string{"error": "api version mismatch"})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithRequestIDGenerator(sequenceRequestID("rid-v", "rid-upload")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Do(context.Background(), Request{
		Method:       "POST",
		Path:         "/api/storage/write",
		Body:         ReaderBody(strings.NewReader("payload"), "application/octet-stream"),
		ResponseMode: ResponseModeJSON,
	})
	var versionErr *VersionError
	if !errors.As(err, &versionErr) {
		t.Fatalf("error = %T %v, want VersionError", err, err)
	}
	if uploadCalls != 1 {
		t.Fatalf("uploadCalls = %d, want no retry", uploadCalls)
	}
}

func TestProxyLocalOnlyGuardRejectsBeforeNetwork(t *testing.T) {
	client, err := NewClient(
		WithEndpointMode(EndpointProxy),
		WithBaseURL("https://api.busy.app"),
		WithCloudBearerToken("cloud-token"),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("network should not be called for local-only proxy operation")
			return nil, nil
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Prepare(context.Background(), Request{
		Method: "POST",
		Path:   "/api/wifi/connect",
	})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %T %v, want ValidationError", err, err)
	}
}

func TestResponseModesAndProtocolError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			writeJSON(t, w, map[string]string{"api_semver": "24.3.0"})
		case "/api/screen":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte{1, 2, 3})
		case "/api/status":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{not-json"))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithRequestIDGenerator(sequenceRequestID("rid-v", "rid-screen", "rid-status")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	resp, err := client.Do(context.Background(), Request{
		Method:       "GET",
		Path:         "/api/screen",
		ResponseMode: ResponseModeBytes,
	})
	if err != nil {
		t.Fatalf("screen Do: %v", err)
	}
	if !bytes.Equal(resp.Body, []byte{1, 2, 3}) {
		t.Fatalf("screen body = %v", resp.Body)
	}

	_, err = client.Do(context.Background(), Request{
		Method:       "GET",
		Path:         "/api/status",
		ResponseMode: ResponseModeJSON,
	})
	var protocolErr *ProtocolError
	if !errors.As(err, &protocolErr) {
		t.Fatalf("error = %T %v, want ProtocolError", err, err)
	}
}

func TestPrepareRejectsUnknownResponseMode(t *testing.T) {
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithRequestIDGenerator(fixedRequestID("rid-1")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Prepare(context.Background(), Request{
		Method:       "GET",
		Path:         "/api/status",
		ResponseMode: ResponseMode("xml"),
	})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %T %v, want ValidationError", err, err)
	}
}

func TestResponseModeTextDoesNotValidateJSON(t *testing.T) {
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithRequestIDGenerator(fixedRequestID("rid-1")),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader("plain text")),
			}, nil
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.setCachedAPISemVerForTest("24.3.0")

	resp, err := client.Do(context.Background(), Request{
		Method:       "GET",
		Path:         "/api/log_dump",
		ResponseMode: ResponseModeText,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if string(resp.Body) != "plain text" {
		t.Fatalf("Body = %q", resp.Body)
	}
}

func TestAPIErrorPreservesRequestContextAndExcerpt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			writeJSON(t, w, map[string]string{"api_semver": "24.3.0"})
		case "/api/display/draw":
			w.Header().Set("X-Request-ID", "device-rid")
			w.WriteHeader(http.StatusConflict)
			writeJSON(t, w, map[string]any{
				"error": "priority too low",
				"code":  409,
			})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithRequestIDGenerator(sequenceRequestID("rid-v", "rid-draw")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Do(context.Background(), Request{
		Method:       "POST",
		Path:         "/api/display/draw",
		Body:         JSONBody(map[string]string{"application_name": "app"}),
		ResponseMode: ResponseModeJSON,
	})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v, want APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusConflict {
		t.Fatalf("StatusCode = %d", apiErr.StatusCode)
	}
	if apiErr.Method != http.MethodPost || apiErr.Path != "/api/display/draw" {
		t.Fatalf("method/path = %s %s", apiErr.Method, apiErr.Path)
	}
	if apiErr.RequestID != "device-rid" {
		t.Fatalf("RequestID = %q", apiErr.RequestID)
	}
	if apiErr.DeviceError != "priority too low" {
		t.Fatalf("DeviceError = %q", apiErr.DeviceError)
	}
	if !strings.Contains(apiErr.Excerpt, "priority too low") {
		t.Fatalf("Excerpt = %q", apiErr.Excerpt)
	}
}

func TestTransportRetryUsesRepeatableBodiesOnly(t *testing.T) {
	attempts := 0
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithRetryPolicy(RetryPolicy{MaxAttempts: 2}),
		WithRequestIDGenerator(fixedRequestID("rid-1")),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return nil, io.ErrUnexpectedEOF
			}
			if r.Body == nil {
				t.Fatal("expected request body")
			}
			return jsonResponse(200, map[string]string{"ok": "true"}), nil
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.setCachedAPISemVerForTest("24.3.0")

	if _, err := client.Do(context.Background(), Request{
		Method:       "POST",
		Path:         "/api/storage/write",
		Body:         BytesBody([]byte("payload"), "application/octet-stream"),
		ResponseMode: ResponseModeJSON,
	}); err != nil {
		t.Fatalf("Do repeatable: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want retry", attempts)
	}

	attempts = 0
	client, err = NewClient(
		WithBaseURL("http://busybar.local"),
		WithRetryPolicy(RetryPolicy{MaxAttempts: 2}),
		WithRequestIDGenerator(fixedRequestID("rid-1")),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			attempts++
			return nil, io.ErrUnexpectedEOF
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient non-repeatable: %v", err)
	}
	client.setCachedAPISemVerForTest("24.3.0")

	_, err = client.Do(context.Background(), Request{
		Method:       "POST",
		Path:         "/api/storage/write",
		Body:         ReaderBody(strings.NewReader("payload"), "application/octet-stream"),
		ResponseMode: ResponseModeJSON,
	})
	var requestErr *RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("error = %T %v, want RequestError", err, err)
	}
	if attempts != 1 {
		t.Fatalf("non-repeatable attempts = %d, want 1", attempts)
	}
}

func TestDoPreparedDoesNotMutatePreparedHeaders(t *testing.T) {
	var versionCalls int
	var drawCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			versionCalls++
			writeJSON(t, w, map[string]string{"api_semver": "24.3." + string(rune('0'+versionCalls))})
		case "/api/display/draw":
			drawCalls++
			if drawCalls == 1 {
				w.WriteHeader(http.StatusMethodNotAllowed)
				writeJSON(t, w, map[string]string{"error": "api version mismatch"})
				return
			}
			writeJSON(t, w, map[string]string{"draw": "ok"})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithRequestIDGenerator(sequenceRequestID("rid-draw", "rid-v1", "rid-v2")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	prepared, err := client.Prepare(context.Background(), Request{
		Method:       "POST",
		Path:         "/api/display/draw",
		Body:         JSONBody(map[string]string{"application_name": "app"}),
		ResponseMode: ResponseModeJSON,
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if _, err := client.DoPrepared(context.Background(), prepared); err != nil {
		t.Fatalf("DoPrepared: %v", err)
	}
	if got := prepared.Header.Get("X-API-Sem-Ver"); got != "" {
		t.Fatalf("prepared X-API-Sem-Ver = %q, want unchanged empty header", got)
	}
	if drawCalls != 2 {
		t.Fatalf("drawCalls = %d, want retry", drawCalls)
	}
}

func TestAPISemVerCoalescesConcurrentFirstUse(t *testing.T) {
	var versionCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			versionCalls.Add(1)
			time.Sleep(20 * time.Millisecond)
			writeJSON(t, w, map[string]string{"api_semver": "24.3.0"})
		case "/api/status":
			writeJSON(t, w, map[string]string{"status": "ok"})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithRequestIDGenerator(fixedRequestID("rid-1")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.Do(context.Background(), Request{
				Method:       "GET",
				Path:         "/api/status",
				ResponseMode: ResponseModeJSON,
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
	}
	if got := versionCalls.Load(); got != 1 {
		t.Fatalf("versionCalls = %d, want 1", got)
	}
}

func TestRefreshAPISemVerForcesVersionRequest(t *testing.T) {
	var versionCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		versionCalls++
		writeJSON(t, w, map[string]string{"api_semver": "24.3." + string(rune('0'+versionCalls))})
	}))
	defer server.Close()

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithRequestIDGenerator(sequenceRequestID("rid-v1", "rid-v2")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	first, err := client.APISemVer(context.Background())
	if err != nil {
		t.Fatalf("APISemVer: %v", err)
	}
	second, err := client.RefreshAPISemVer(context.Background())
	if err != nil {
		t.Fatalf("RefreshAPISemVer: %v", err)
	}
	if first == second {
		t.Fatalf("RefreshAPISemVer returned %q, want a fresh value", second)
	}
	if versionCalls != 2 {
		t.Fatalf("versionCalls = %d, want 2", versionCalls)
	}
}

func TestVersionNegotiationCanBeDisabled(t *testing.T) {
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithRequestIDGenerator(fixedRequestID("rid-1")),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("X-API-Sem-Ver"); got != "" {
				t.Fatalf("X-API-Sem-Ver = %q, want absent", got)
			}
			return jsonResponse(http.StatusOK, map[string]string{"status": "ok"}), nil
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := client.Do(context.Background(), Request{
		Method:       "GET",
		Path:         "/api/status",
		ResponseMode: ResponseModeJSON,
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
}

func TestContextCancellationReturnsRequestError(t *testing.T) {
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithRequestIDGenerator(fixedRequestID("rid-1")),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			<-r.Context().Done()
			return nil, r.Context().Err()
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.setCachedAPISemVerForTest("24.3.0")

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	_, err = client.Do(ctx, Request{
		Method:       "GET",
		Path:         "/api/status",
		ResponseMode: ResponseModeJSON,
	})
	var requestErr *RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("error = %T %v, want RequestError", err, err)
	}
}

func fixedRequestID(id string) func() string {
	return func() string { return id }
}

func sequenceRequestID(ids ...string) func() string {
	index := 0
	return func() string {
		if index >= len(ids) {
			return ids[len(ids)-1]
		}
		id := ids[index]
		index++
		return id
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func jsonResponse(status int, payload any) *http.Response {
	var buffer bytes.Buffer
	_ = json.NewEncoder(&buffer).Encode(payload)
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(&buffer),
	}
}
