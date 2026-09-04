package busylib

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestRevokeAccessTokenValidatesShortIDBeforeTransport(t *testing.T) {
	requests := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("unexpected transport call")
	})}

	client, err := NewClient(
		WithBaseURL("http://busybar.local.invalid"),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithHTTPClient(httpClient),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	invalidShortIDs := []string{
		"",
		"AAMTBO0",
		"AAMTBO0f1",
		"AAMTBO-f",
		"AAMTBÖOf",
	}
	for _, shortID := range invalidShortIDs {
		t.Run(shortID, func(t *testing.T) {
			err := client.Settings().RevokeAccessToken(context.Background(), shortID)
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("RevokeAccessToken error = %T %v, want ValidationError", err, err)
			}
		})
	}
	if requests != 0 {
		t.Fatalf("transport calls = %d, want 0", requests)
	}
}
