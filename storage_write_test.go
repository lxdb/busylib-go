package busylib

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStorageWriteEmitsAppendQueryForAllPointerStates(t *testing.T) {
	appendFalse := false
	appendTrue := true
	tests := []struct {
		name   string
		append *bool
		query  string
	}{
		{name: "nil omits append", query: "path=%2Fext%2Fpayload.bin"},
		{name: "false sends zero", append: &appendFalse, query: "append=0&path=%2Fext%2Fpayload.bin"},
		{name: "true sends one", append: &appendTrue, query: "append=1&path=%2Fext%2Fpayload.bin"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/storage/write" || r.URL.RawQuery != test.query {
					t.Errorf("request = %s %s?%s, want POST /api/storage/write?%s", r.Method, r.URL.Path, r.URL.RawQuery, test.query)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read body: %v", err)
				}
				if string(body) != "payload" {
					t.Errorf("body = %q, want payload", body)
				}
				writeJSON(t, w, map[string]string{"result": "OK"})
			}))
			defer server.Close()

			client, err := NewClient(
				WithBaseURL(server.URL),
				WithHTTPClient(server.Client()),
				WithVersionNegotiation(VersionNegotiationDisabled),
			)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			err = client.Storage().Write(context.Background(), WriteStorageFileRequest{
				Path:   "/ext/payload.bin",
				Body:   BytesBody([]byte("payload"), "application/octet-stream"),
				Append: test.append,
			})
			if err != nil {
				t.Fatalf("Write: %v", err)
			}
		})
	}
}
