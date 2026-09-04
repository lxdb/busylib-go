package busylib

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestApplicationNameValidationBoundaries(t *testing.T) {
	maximum := "Aa0._-" + strings.Repeat("b", 25)
	if len(maximum) != maxApplicationNameBytes {
		t.Fatalf("maximum fixture length = %d, want %d", len(maximum), maxApplicationNameBytes)
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "minimum", value: "a"},
		{name: "all supported characters at maximum", value: maximum},
		{name: "empty", value: "", wantErr: true},
		{name: "above maximum", value: maximum + "b", wantErr: true},
		{name: "space", value: "my app", wantErr: true},
		{name: "non-ASCII", value: "aplicacion-ñ", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateApplicationName(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateApplicationName(%q) error = %v, wantErr %t", test.value, err, test.wantErr)
			}
		})
	}
}

func TestUploadedAssetPathValidationBoundaries(t *testing.T) {
	maximum := "Dir_1-A/File.B-2/" + strings.Repeat("x", 43) + ".png"
	if len(maximum) != maxUploadedAssetPathBytes {
		t.Fatalf("maximum fixture length = %d, want %d", len(maximum), maxUploadedAssetPathBytes)
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "nested path", value: "themes/meeting/icon.png"},
		{name: "maximum length", value: maximum},
		{name: "empty", value: "", wantErr: true},
		{name: "above maximum", value: maximum + "x", wantErr: true},
		{name: "leading slash", value: "/image.png", wantErr: true},
		{name: "trailing slash", value: "images/", wantErr: true},
		{name: "empty segment", value: "images//icon.png", wantErr: true},
		{name: "consecutive dots", value: "images/icon..png", wantErr: true},
		{name: "space", value: "images/my icon.png", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateUploadedAssetPath("path", test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateUploadedAssetPath(%q) error = %v, wantErr %t", test.value, err, test.wantErr)
			}
		})
	}
}

func TestStockAssetPathValidationBoundaries(t *testing.T) {
	maximum := "shared/Dir_1-A/File.B-2/" + strings.Repeat("x", 228) + ".snd"
	if len(maximum) != maxStockAssetPathBytes {
		t.Fatalf("maximum fixture length = %d, want %d", len(maximum), maxStockAssetPathBytes)
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "nested shared path", value: "shared/audio/notifications/tone.snd"},
		{name: "maximum length", value: maximum},
		{name: "wrong prefix", value: "private/tone.snd", wantErr: true},
		{name: "above maximum", value: maximum + "x", wantErr: true},
		{name: "trailing slash", value: "shared/audio/", wantErr: true},
		{name: "empty segment", value: "shared//tone.snd", wantErr: true},
		{name: "consecutive dots", value: "shared/tone..snd", wantErr: true},
		{name: "traversal", value: "shared/audio/../tone.snd", wantErr: true},
		{name: "space", value: "shared/my tone.snd", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateStockAssetPath("stock_path", test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateStockAssetPath(%q) error = %v, wantErr %t", test.value, err, test.wantErr)
			}
		})
	}
}

func TestServicesRejectInvalidApplicationNamesBeforeTransport(t *testing.T) {
	client, requests := clientThatCountsRequests(t)
	ctx := context.Background()
	const invalid = "my app"
	tests := []validationCall{
		{name: "draw", call: func() error {
			return client.Display().Draw(ctx, NewDisplayElements(invalid, NewTextElement("text", "hello", FontNormal)))
		}},
		{name: "audio", call: func() error { return client.Audio().PlayAsset(ctx, invalid, "tone.snd") }},
		{name: "upload", call: func() error {
			return client.Assets().Upload(ctx, UploadAssetRequest{
				ApplicationName: invalid,
				File:            "image.png",
				Body:            BytesBody([]byte("asset"), "application/octet-stream"),
			})
		}},
		{name: "delete application assets", call: func() error {
			return client.Assets().DeleteApplicationAssets(ctx, invalid)
		}},
		{name: "clear", call: func() error { return client.Display().Clear(ctx, invalid) }},
		{name: "clear elements", call: func() error {
			return client.Display().ClearElements(ctx, ClearDisplayElementsRequest{
				ApplicationName: invalid,
				ElementIDs:      []string{"element"},
			})
		}},
	}
	assertCallsFailValidation(t, tests)
	if got := requests.Load(); got != 0 {
		t.Fatalf("transport calls = %d, want 0", got)
	}
}

func TestServicesRejectInvalidAssetPathsBeforeTransport(t *testing.T) {
	client, requests := clientThatCountsRequests(t)
	ctx := context.Background()
	tests := []validationCall{
		{name: "upload", call: func() error {
			return client.Assets().Upload(ctx, UploadAssetRequest{
				ApplicationName: "app",
				File:            "images//icon.png",
				Body:            BytesBody([]byte("asset"), "application/octet-stream"),
			})
		}},
		{name: "image", call: func() error {
			return client.Display().Draw(ctx, NewDisplayElements("app", NewAssetImageElement("image", "images//icon.png")))
		}},
		{name: "animation", call: func() error {
			return client.Display().Draw(ctx, NewDisplayElements("app", NewAssetAnimationElement("animation", "images//animation.gif")))
		}},
		{name: "audio", call: func() error { return client.Audio().PlayAsset(ctx, "app", "audio//tone.snd") }},
		{name: "stock image", call: func() error {
			return client.Display().Draw(ctx, NewDisplayElements("app", NewStockImageElement("image", "private/image.png")))
		}},
		{name: "stock animation", call: func() error {
			return client.Display().Draw(ctx, NewDisplayElements("app", NewStockAnimationElement("animation", "private/animation.gif")))
		}},
		{name: "stock audio", call: func() error { return client.Audio().PlayStock(ctx, "app", "private/tone.snd") }},
	}
	assertCallsFailValidation(t, tests)
	if got := requests.Load(); got != 0 {
		t.Fatalf("transport calls = %d, want 0", got)
	}
}

type validationCall struct {
	name string
	call func() error
}

func clientThatCountsRequests(t *testing.T) (*Client, *atomic.Int64) {
	t.Helper()
	var requests atomic.Int64
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
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
	return client, &requests
}

func assertCallsFailValidation(t *testing.T, tests []validationCall) {
	t.Helper()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("error = %T %v, want ValidationError", err, err)
			}
		})
	}
}
