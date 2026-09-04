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
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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

	prepared, err := client.Prepare(Request{
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

	if prepared.Method() != http.MethodGet {
		t.Fatalf("Method = %q", prepared.Method())
	}
	preparedURL := prepared.URL()
	if got := preparedURL.String(); got != "http://busybar.local/api/status?display=front" {
		t.Fatalf("URL = %q", got)
	}
	if got := prepared.Header().Get("X-API-Token"); got != "1234" {
		t.Fatalf("X-API-Token = %q", got)
	}
	if got := prepared.Header().Get("X-Request-ID"); got != "rid-1" {
		t.Fatalf("X-Request-ID = %q", got)
	}
	if got := prepared.Header().Get("x-session-id"); got != "request-session" {
		t.Fatalf("x-session-id = %q", got)
	}
	if got := prepared.RequestID(); got != "rid-1" {
		t.Fatalf("RequestID = %q", got)
	}
}

func TestLocalAccessCredentialOptionsUseAPITokenHeaderAndLastWins(t *testing.T) {
	tests := []struct {
		name    string
		options []Option
		want    string
	}{
		{name: "token", options: []Option{WithLocalAccessToken("token-secret")}, want: "token-secret"},
		{name: "key", options: []Option{WithLocalAccessKey("key-secret")}, want: "key-secret"},
		{name: "token after key", options: []Option{WithLocalAccessKey("key-secret"), WithLocalAccessToken("token-secret")}, want: "token-secret"},
		{name: "key after token", options: []Option{WithLocalAccessToken("token-secret"), WithLocalAccessKey("key-secret")}, want: "key-secret"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			receivedToken := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/status" {
					t.Errorf("request = %s %s, want GET /api/status", r.Method, r.URL.Path)
				}
				receivedToken <- r.Header.Get("X-API-Token")
				writeJSON(t, w, map[string]string{"status": "ok"})
			}))
			defer server.Close()

			client, err := NewClient(append(test.options,
				WithBaseURL(server.URL),
				WithHTTPClient(server.Client()),
				WithVersionNegotiation(VersionNegotiationDisabled),
				WithRequestIDGenerator(fixedRequestID("rid-1")),
			)...)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}

			if _, err := client.Do(context.Background(), Request{
				Method:       http.MethodGet,
				Path:         "/api/status",
				ResponseMode: ResponseModeJSON,
			}); err != nil {
				t.Fatalf("Do: %v", err)
			}
			if got := <-receivedToken; got != test.want {
				t.Fatalf("X-API-Token = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPreparedRequestAccessorsCannotChangeExecution(t *testing.T) {
	var receivedPath string
	var receivedToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedToken = r.Header.Get("X-API-Token")
		writeJSON(t, w, map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithLocalAccessKey("secret"),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithRequestIDGenerator(fixedRequestID("rid-1")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	prepared, err := client.Prepare(Request{
		Method:       http.MethodGet,
		Path:         "/api/status",
		ResponseMode: ResponseModeJSON,
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	viewURL := prepared.URL()
	viewURL.Host = "attacker.invalid"
	viewURL.Path = "/stolen"
	viewHeader := prepared.Header()
	viewHeader.Set("X-API-Token", "changed")

	if _, err := client.DoPrepared(context.Background(), prepared); err != nil {
		t.Fatalf("DoPrepared: %v", err)
	}
	if receivedPath != "/api/status" {
		t.Fatalf("request path = %q, want /api/status", receivedPath)
	}
	if receivedToken != "secret" {
		t.Fatalf("X-API-Token = %q, want original token", receivedToken)
	}
	if prepared.Method() != http.MethodGet || prepared.Path() != "/api/status" {
		t.Fatalf("prepared method/path = %s %s", prepared.Method(), prepared.Path())
	}
	if prepared.ResponseMode() != ResponseModeJSON || prepared.RequestID() != "rid-1" {
		t.Fatalf("prepared response mode/request ID = %v %q", prepared.ResponseMode(), prepared.RequestID())
	}
}

func TestRemoteModePreservesCanonicalPathAndDoesNotInjectAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			writeJSON(t, w, map[string]string{"api_semver": "25.0.0"})
			return
		}
		if r.URL.Path != "/api/status" {
			t.Errorf("path = %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want absent", got)
		}
		if got := r.Header.Get("X-API-Token"); got != "" {
			t.Errorf("X-API-Token = %q, want absent", got)
		}
		if got := r.Header.Get("X-API-Sem-Ver"); got != "25.0.0" {
			t.Errorf("X-API-Sem-Ver = %q", got)
		}
		writeJSON(t, w, map[string]string{"ok": "true"})
	}))
	defer server.Close()

	client, err := NewClient(
		WithEndpointMode(EndpointRemote),
		WithBaseURL(server.URL+"/ignored"),
		WithHTTPClient(server.Client()),
		WithRequestIDGenerator(sequenceRequestID("version-rid", "request-rid")),
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

func TestRemoteModeRequiresExplicitTransportAndRejectsLocalCredentials(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("unused")
	})}
	tests := []struct {
		name    string
		options []Option
		want    string
	}{
		{name: "base URL", options: []Option{WithEndpointMode(EndpointRemote), WithHTTPClient(httpClient)}},
		{name: "HTTP client", options: []Option{WithEndpointMode(EndpointRemote), WithBaseURL("http://busybar.remote.invalid")}},
		{name: "local access token", options: []Option{WithEndpointMode(EndpointRemote), WithBaseURL("http://busybar.remote.invalid"), WithHTTPClient(httpClient), WithLocalAccessToken("secret")}, want: "WithLocalAccessToken"},
		{name: "local access key", options: []Option{WithEndpointMode(EndpointRemote), WithBaseURL("http://busybar.remote.invalid"), WithHTTPClient(httpClient), WithLocalAccessKey("1234")}, want: "WithLocalAccessToken"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewClient(test.options...)
			if err == nil {
				t.Fatal("NewClient succeeded")
			}
			if test.want != "" && !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewClient error = %q, want guidance containing %q", err, test.want)
			}
		})
	}
	if _, err := NewClient(
		WithEndpointMode(EndpointRemote),
		WithBaseURL("http://busybar.remote.invalid"),
		WithHTTPClient(httpClient),
	); err != nil {
		t.Fatalf("NewClient with explicit remote transport: %v", err)
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
	_, err = local.Prepare(Request{
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

	remote, err := NewClient(
		WithEndpointMode(EndpointRemote),
		WithBaseURL("http://busybar.remote.invalid"),
		WithHTTPClient(&http.Client{}),
	)
	if err != nil {
		t.Fatalf("remote NewClient: %v", err)
	}
	_, err = remote.Prepare(Request{
		Method: "GET",
		Path:   "/api/status",
		Header: http.Header{
			"X-API-Token": []string{"1234"},
		},
	})
	if !errors.As(err, &validationErr) {
		t.Fatalf("remote conflicting auth error = %T %v, want ValidationError", err, err)
	}
	if !strings.Contains(err.Error(), "WithLocalAccessToken") {
		t.Fatalf("remote conflicting auth error = %q, want WithLocalAccessToken guidance", err)
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
				t.Errorf("version request X-API-Sem-Ver = %q", got)
			}
			writeJSON(t, w, map[string]string{"api_semver": "25.0.0"})
		case "/api/status":
			statusCalls++
			if got := r.Header.Get("X-API-Sem-Ver"); got != "25.0.0" {
				t.Errorf("status request X-API-Sem-Ver = %q", got)
			}
			writeJSON(t, w, map[string]string{"status": "ok"})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
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
			writeJSON(t, w, map[string]string{"api_semver": "24.4." + string(rune('0'+versionCalls))})
		case "/api/display/draw":
			drawCalls++
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read body: %v", err)
				return
			}
			bodies = append(bodies, string(body))
			if drawCalls == 1 {
				w.WriteHeader(http.StatusMethodNotAllowed)
				writeJSON(t, w, map[string]string{"error": "api version mismatch"})
				return
			}
			if got := r.Header.Get("X-API-Sem-Ver"); got != "24.4.2" {
				t.Errorf("retried X-API-Sem-Ver = %q", got)
			}
			writeJSON(t, w, map[string]string{"draw": "ok"})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
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
			writeJSON(t, w, map[string]string{"api_semver": "25.0.0"})
		case "/api/storage/write":
			uploadCalls++
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(t, w, map[string]string{"error": "api version mismatch"})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
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

func TestFileBodyCompatibilityRetryReplaysFile(t *testing.T) {
	tempDir := t.TempDir()
	localPath := filepath.Join(tempDir, "payload.bin")
	if err := os.WriteFile(localPath, []byte("file payload"), 0o600); err != nil {
		t.Fatalf("WriteFile fixture: %v", err)
	}

	var versionCalls int
	var writeCalls int
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			versionCalls++
			writeJSON(t, w, map[string]string{"api_semver": "24.4." + string(rune('0'+versionCalls))})
		case "/api/storage/write":
			writeCalls++
			if r.ContentLength != int64(len("file payload")) {
				t.Errorf("ContentLength = %d", r.ContentLength)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read body: %v", err)
				return
			}
			bodies = append(bodies, string(body))
			if writeCalls == 1 {
				w.WriteHeader(http.StatusMethodNotAllowed)
				writeJSON(t, w, map[string]string{"error": "api version mismatch"})
				return
			}
			if got := r.Header.Get("X-API-Sem-Ver"); got != "24.4.2" {
				t.Errorf("retried X-API-Sem-Ver = %q", got)
			}
			writeJSON(t, w, map[string]string{"result": "OK"})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithRequestIDGenerator(sequenceRequestID("rid-v1", "rid-write", "rid-v2")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := client.Do(context.Background(), Request{
		Method:       http.MethodPost,
		Path:         "/api/storage/write",
		Body:         FileBody(localPath, "application/octet-stream"),
		ResponseMode: ResponseModeJSON,
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if versionCalls != 2 {
		t.Fatalf("versionCalls = %d, want 2", versionCalls)
	}
	if writeCalls != 2 {
		t.Fatalf("writeCalls = %d, want 2", writeCalls)
	}
	if len(bodies) != 2 || bodies[0] != "file payload" || bodies[1] != "file payload" {
		t.Fatalf("bodies = %#v", bodies)
	}
}

func TestStreamedCompatibilityRetryRefreshesVersionAndCopiesBody(t *testing.T) {
	var versionCalls int
	var readCalls int
	var readSemVers []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			versionCalls++
			writeJSON(t, w, map[string]string{"api_semver": "24.4." + string(rune('0'+versionCalls))})
		case "/api/storage/read":
			readCalls++
			readSemVers = append(readSemVers, r.Header.Get("X-API-Sem-Ver"))
			if readCalls == 1 {
				w.WriteHeader(http.StatusMethodNotAllowed)
				writeJSON(t, w, map[string]string{"error": "api version mismatch"})
				return
			}
			_, _ = w.Write([]byte("streamed payload"))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithRequestIDGenerator(sequenceRequestID("rid-read", "rid-v1", "rid-v2")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	var out bytes.Buffer
	n, err := client.Storage().ReadTo(context.Background(), "/ext/payload.bin", &out)
	if err != nil {
		t.Fatalf("ReadTo: %v", err)
	}
	if n != int64(len("streamed payload")) || out.String() != "streamed payload" {
		t.Fatalf("ReadTo wrote n=%d body=%q", n, out.String())
	}
	if versionCalls != 2 {
		t.Fatalf("versionCalls = %d, want 2", versionCalls)
	}
	if readCalls != 2 {
		t.Fatalf("readCalls = %d, want 2", readCalls)
	}
	wantSemVers := []string{"24.4.1", "24.4.2"}
	if !reflect.DeepEqual(readSemVers, wantSemVers) {
		t.Fatalf("read semver headers = %#v, want %#v", readSemVers, wantSemVers)
	}
}

func TestStreamedCompatibilityRetryReturnsBodyReadError(t *testing.T) {
	bodyErr := errors.New("compatibility body read failed")
	var calls int
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			switch calls {
			case 1:
				if r.Method != http.MethodGet || r.URL.Path != "/api/storage/read" {
					t.Fatalf("request = %s %s", r.Method, r.URL.Path)
				}
				return &http.Response{
					StatusCode: http.StatusMethodNotAllowed,
					Status:     http.StatusText(http.StatusMethodNotAllowed),
					Header:     http.Header{"X-Request-ID": []string{"device-rid"}},
					Body:       errorReadCloser{err: bodyErr},
				}, nil
			case 2:
				return jsonResponse(http.StatusOK, map[string]string{"api_semver": "24.4.1"}), nil
			default:
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader("unexpected retry")),
				}, nil
			}
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	var out bytes.Buffer
	n, err := client.Storage().ReadTo(context.Background(), "/ext/payload.bin", &out)
	var requestErr *RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("error = %T %v, want RequestError", err, err)
	}
	if !errors.Is(requestErr.Err, bodyErr) {
		t.Fatalf("RequestError.Err = %v, want %v", requestErr.Err, bodyErr)
	}
	if requestErr.RequestID != "device-rid" {
		t.Fatalf("RequestID = %q, want device response request ID", requestErr.RequestID)
	}
	if n != 0 || out.Len() != 0 {
		t.Fatalf("ReadTo wrote n=%d body=%q before failing", n, out.String())
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want no refresh or retry after compatibility body read failure", calls)
	}
}

func TestProgressBodyReportsKnownAndUnknownTotals(t *testing.T) {
	t.Run("known total", func(t *testing.T) {
		var progress []struct {
			written int64
			total   int64
		}
		client, err := NewClient(
			WithBaseURL("http://busybar.local"),
			WithVersionNegotiation(VersionNegotiationDisabled),
			WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				if string(body) != "payload" {
					t.Fatalf("body = %q", body)
				}
				return jsonResponse(http.StatusOK, map[string]string{"result": "OK"}), nil
			})}),
		)
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		err = client.Storage().Write(context.Background(), WriteStorageFileRequest{
			Path: "/ext/payload.bin",
			Body: ProgressBody(BytesBody([]byte("payload"), "application/octet-stream"), func(written, total int64) {
				progress = append(progress, struct {
					written int64
					total   int64
				}{written: written, total: total})
			}),
		})
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		if len(progress) == 0 {
			t.Fatal("progress callback was not called")
		}
		last := progress[len(progress)-1]
		if last.written != int64(len("payload")) || last.total != int64(len("payload")) {
			t.Fatalf("last progress = %+v", last)
		}
		for i := 1; i < len(progress); i++ {
			if progress[i].written < progress[i-1].written {
				t.Fatalf("progress regressed at %d: %#v", i, progress)
			}
		}
	})

	t.Run("unknown total", func(t *testing.T) {
		var lastTotal int64
		client, err := NewClient(
			WithBaseURL("http://busybar.local"),
			WithVersionNegotiation(VersionNegotiationDisabled),
			WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				_, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				return jsonResponse(http.StatusOK, map[string]string{"result": "OK"}), nil
			})}),
		)
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		err = client.Storage().Write(context.Background(), WriteStorageFileRequest{
			Path: "/ext/payload.bin",
			Body: ProgressBody(ReaderBody(strings.NewReader("stream"), "application/octet-stream"), func(_, total int64) {
				lastTotal = total
			}),
		})
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		if lastTotal != -1 {
			t.Fatalf("last total = %d, want -1", lastTotal)
		}
	})

	t.Run("failed upload is not replayed", func(t *testing.T) {
		var progress []int64
		attempts := 0
		client, err := NewClient(
			WithBaseURL("http://busybar.local"),
			WithVersionNegotiation(VersionNegotiationDisabled),
			WithRetryPolicy(RetryPolicy{MaxAttempts: 2}),
			WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				attempts++
				buffer := make([]byte, 3)
				for {
					_, readErr := r.Body.Read(buffer)
					if readErr == io.EOF {
						break
					}
					if readErr != nil {
						return nil, readErr
					}
				}
				_ = r.Body.Close()
				return nil, io.ErrUnexpectedEOF
			})}),
		)
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		err = client.Assets().Upload(context.Background(), UploadAssetRequest{
			ApplicationName: "app",
			File:            "payload.bin",
			Body: ProgressBody(BytesBody([]byte("payload"), "application/octet-stream"), func(written, _ int64) {
				progress = append(progress, written)
			}),
		})
		var requestErr *RequestError
		if !errors.As(err, &requestErr) {
			t.Fatalf("Upload error = %T %v, want RequestError", err, err)
		}
		want := []int64{3, 6, 7}
		if !reflect.DeepEqual(progress, want) {
			t.Fatalf("progress = %#v, want %#v", progress, want)
		}
		if attempts != 1 {
			t.Fatalf("attempts = %d, want 1", attempts)
		}
	})
}

func TestRemoteBlocklistGuardRejectsBeforeNetwork(t *testing.T) {
	client, err := NewClient(
		WithEndpointMode(EndpointRemote),
		WithBaseURL("http://busybar.remote.invalid"),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("network should not be called for firmware-blocked remote operation")
			return nil, nil
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Prepare(Request{
		Method: http.MethodPost,
		Path:   "/api/update",
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
			writeJSON(t, w, map[string]string{"api_semver": "25.0.0"})
		case "/api/screen":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte{1, 2, 3})
		case "/api/status":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{not-json"))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
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

	_, err = client.Prepare(Request{
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
		WithVersionNegotiation(VersionNegotiationDisabled),
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

func TestBufferedResponseHonorsConfiguredLimit(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr error
	}{
		{name: "at limit", body: "1234"},
		{name: "over limit", body: "12345", wantErr: ErrResponseTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := &trackingReadCloser{Reader: strings.NewReader(test.body)}
			client, err := NewClient(
				WithVersionNegotiation(VersionNegotiationDisabled),
				WithMaxResponseBytes(4),
				WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       body,
					}, nil
				})}),
			)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}

			response, err := client.Do(context.Background(), Request{
				Method:       http.MethodGet,
				Path:         "/api/status",
				ResponseMode: ResponseModeText,
			})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Do error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr == nil && string(response.Body) != test.body {
				t.Fatalf("Body = %q, want %q", response.Body, test.body)
			}
			if !body.closed {
				t.Fatal("response body was not closed")
			}
		})
	}
}

func TestWithMaxResponseBytesRejectsNonPositiveValues(t *testing.T) {
	for _, maximum := range []int64{-1, 0} {
		if _, err := NewClient(WithMaxResponseBytes(maximum)); err == nil {
			t.Fatalf("WithMaxResponseBytes(%d) succeeded", maximum)
		}
	}
}

func TestExecutionRejectsNilContextAndMalformedPreparedRequest(t *testing.T) {
	client, err := NewClient(
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]string{"status": "ok"}), nil
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := client.Do(nil, Request{Method: http.MethodGet, Path: "/api/status"}); err == nil { //nolint:staticcheck // Verifies nil rejection.
		t.Fatal("Do accepted a nil context")
	}
	if _, err := client.DoPrepared(context.Background(), &PreparedRequest{}); err == nil {
		t.Fatal("DoPrepared accepted a malformed prepared request")
	}
	prepared, err := client.Prepare(Request{Method: http.MethodGet, Path: "/api/status"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if _, err := client.DoPrepared(context.Background(), prepared); err != nil {
		t.Fatalf("DoPrepared: %v", err)
	}
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestAPIErrorPreservesRequestContextAndExcerpt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			writeJSON(t, w, map[string]string{"api_semver": "25.0.0"})
		case "/api/display/draw":
			w.Header().Set("X-Request-ID", "device-rid")
			w.WriteHeader(http.StatusConflict)
			writeJSON(t, w, map[string]any{
				"error": "priority too low",
				"code":  409,
			})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
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

func TestTransportRetryUsesSafeMethodsAndRepeatableBodiesOnly(t *testing.T) {
	attempts := 0
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithVersionNegotiation(VersionNegotiationDisabled),
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
	_, err = client.Do(context.Background(), Request{
		Method:       "POST",
		Path:         "/api/storage/write",
		Body:         BytesBody([]byte("payload"), "application/octet-stream"),
		ResponseMode: ResponseModeJSON,
	})
	var requestErr *RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("repeatable POST error = %T %v, want RequestError", err, err)
	}
	if attempts != 1 {
		t.Fatalf("repeatable POST attempts = %d, want 1", attempts)
	}

	attempts = 0
	client, err = NewClient(
		WithBaseURL("http://busybar.local"),
		WithVersionNegotiation(VersionNegotiationDisabled),
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
	_, err = client.Do(context.Background(), Request{
		Method:       http.MethodGet,
		Path:         "/api/status",
		ResponseMode: ResponseModeJSON,
	})
	if !errors.As(err, &requestErr) {
		t.Fatalf("repeatable GET error = %T %v, want RequestError", err, err)
	}
	if attempts != 2 {
		t.Fatalf("repeatable GET attempts = %d, want 2", attempts)
	}
}

func TestTransportRetryStopsWhenContextIsCanceledDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithRetryPolicy(RetryPolicy{MaxAttempts: 3, Backoff: time.Hour}),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			attempts++
			cancel()
			return nil, io.ErrUnexpectedEOF
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Do(ctx, Request{Method: http.MethodGet, Path: "/api/status"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do error = %v, want context.Canceled", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestDoPreparedDoesNotMutatePreparedHeaders(t *testing.T) {
	var versionCalls int
	var drawCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			versionCalls++
			writeJSON(t, w, map[string]string{"api_semver": "24.4." + string(rune('0'+versionCalls))})
		case "/api/display/draw":
			drawCalls++
			if drawCalls == 1 {
				w.WriteHeader(http.StatusMethodNotAllowed)
				writeJSON(t, w, map[string]string{"error": "api version mismatch"})
				return
			}
			writeJSON(t, w, map[string]string{"draw": "ok"})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
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
	prepared, err := client.Prepare(Request{
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
	if got := prepared.Header().Get("X-API-Sem-Ver"); got != "" {
		t.Fatalf("prepared X-API-Sem-Ver = %q, want unchanged empty header", got)
	}
	if drawCalls != 2 {
		t.Fatalf("drawCalls = %d, want retry", drawCalls)
	}
}

func TestAPISemVerCoalescesConcurrentFirstUse(t *testing.T) {
	var versionCalls atomic.Int32
	versionStarted := make(chan struct{})
	releaseVersion := make(chan struct{})
	var signalVersion sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			versionCalls.Add(1)
			signalVersion.Do(func() { close(versionStarted) })
			<-releaseVersion
			writeJSON(t, w, map[string]string{"api_semver": "25.0.0"})
		case "/api/status":
			writeJSON(t, w, map[string]string{"status": "ok"})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
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
	ready := make(chan struct{}, 8)
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ready <- struct{}{}
			<-start
			_, err := client.Do(context.Background(), Request{
				Method:       "GET",
				Path:         "/api/status",
				ResponseMode: ResponseModeJSON,
			})
			errs <- err
		}()
	}
	for i := 0; i < 8; i++ {
		<-ready
	}
	close(start)
	<-versionStarted
	close(releaseVersion)
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

func TestAPISemVerCanceledLeaderDoesNotPoisonFollower(t *testing.T) {
	versionStarted := make(chan struct{})
	releaseVersion := make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if calls.Add(1) == 1 {
			close(versionStarted)
		}
		select {
		case <-releaseVersion:
			writeJSON(t, w, map[string]string{"api_semver": "25.0.0"})
		case <-r.Context().Done():
		}
	}))
	defer server.Close()

	client, err := NewClient(WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderResult := make(chan error, 1)
	go func() {
		_, err := client.APISemVer(leaderCtx)
		leaderResult <- err
	}()
	<-versionStarted

	followerResult := make(chan error, 1)
	go func() {
		_, err := client.APISemVer(context.Background())
		followerResult <- err
	}()
	waitForVersionWaiters(t, client, 2)
	cancelLeader()
	if err := <-leaderResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("leader error = %v, want context.Canceled", err)
	}
	close(releaseVersion)
	if err := <-followerResult; err != nil {
		t.Fatalf("follower error = %v, want success", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("version calls = %d, want 1", got)
	}
}

func TestAPISemVerCancelsRefreshAfterAllWaitersLeave(t *testing.T) {
	firstStarted := make(chan struct{})
	firstCanceled := make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if calls.Add(1) == 1 {
			close(firstStarted)
			<-r.Context().Done()
			close(firstCanceled)
			return
		}
		writeJSON(t, w, map[string]string{"api_semver": "25.0.0"})
	}))
	defer server.Close()

	client, err := NewClient(WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	secondCtx, cancelSecond := context.WithCancel(context.Background())
	results := make(chan error, 2)
	go func() {
		_, err := client.APISemVer(firstCtx)
		results <- err
	}()
	<-firstStarted
	go func() {
		_, err := client.APISemVer(secondCtx)
		results <- err
	}()
	waitForVersionWaiters(t, client, 2)
	cancelFirst()
	cancelSecond()
	for range 2 {
		if err := <-results; !errors.Is(err, context.Canceled) {
			t.Fatalf("waiter error = %v, want context.Canceled", err)
		}
	}
	select {
	case <-firstCanceled:
	case <-time.After(time.Second):
		t.Fatal("shared version request was not canceled")
	}

	if got, err := client.APISemVer(context.Background()); err != nil || got != "25.0.0" {
		t.Fatalf("fresh APISemVer = %q, %v; want 25.0.0", got, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("version calls = %d, want 2", got)
	}
}

func waitForVersionWaiters(t *testing.T, client *Client, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		client.versionMu.Lock()
		refresh := client.versionInFlight
		got := 0
		if refresh != nil {
			got = refresh.waiters
		}
		client.versionMu.Unlock()
		if got == want {
			return
		}
		runtime.Gosched()
	}
	t.Fatalf("version refresh did not reach %d waiters", want)
}

func TestRefreshAPISemVerForcesVersionRequest(t *testing.T) {
	var versionCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		versionCalls++
		writeJSON(t, w, map[string]string{"api_semver": "24.4." + string(rune('0'+versionCalls))})
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
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithRequestIDGenerator(fixedRequestID("rid-1")),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			<-r.Context().Done()
			return nil, r.Context().Err()
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
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

type errorReadCloser struct {
	err error
}

func (r errorReadCloser) Read([]byte) (int, error) {
	return 0, r.err
}

func (r errorReadCloser) Close() error {
	return nil
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Errorf("write json: %v", err)
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
