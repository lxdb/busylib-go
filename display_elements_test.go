package busylib

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestXPMBitmapElementValueAndPointerDispatch(t *testing.T) {
	zero := 0
	request := NewDisplayElements("app",
		NewXPMBitmapElement("bitmap.value", "/* XPM */"),
		&XPMBitmapElement{
			BaseDisplayElement: BaseDisplayElement{ID: "bitmap.pointer"},
			Data:               "xpm data",
			Opacity:            &zero,
		},
	)

	if err := request.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	assertJSONEqual(t, string(body), `{
		"application_name":"app",
		"priority":50,
		"elements":[
			{"id":"bitmap.value","display":"front","type":"xpmbitmap","data":"/* XPM */"},
			{"id":"bitmap.pointer","type":"xpmbitmap","data":"xpm data","opacity":0}
		]
	}`)
}

func TestDisplayElementZIndexJSONAndValidationBoundaries(t *testing.T) {
	zero := 0
	maximum := math.MaxInt32
	request := NewDisplayElements("app",
		NewTextElement("nil", "Nil", FontNormal),
		TextElement{BaseDisplayElement: BaseDisplayElement{ID: "zero", ZIndex: &zero}, Text: "Zero", Font: FontNormal},
		TextElement{BaseDisplayElement: BaseDisplayElement{ID: "maximum", ZIndex: &maximum}, Text: "Maximum", Font: FontNormal},
	)

	if err := request.Validate(); err != nil {
		t.Fatalf("Validate boundary values: %v", err)
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	assertJSONEqual(t, string(body), `{
		"application_name":"app",
		"priority":50,
		"elements":[
			{"id":"nil","display":"front","type":"text","text":"Nil","font":"normal"},
			{"id":"zero","z_index":0,"type":"text","text":"Zero","font":"normal"},
			{"id":"maximum","z_index":2147483647,"type":"text","text":"Maximum","font":"normal"}
		]
	}`)

	invalid := []int{-1}
	if strconv.IntSize > 32 {
		invalid = append(invalid, int(int64(math.MaxInt32)+1))
	}
	for _, zIndex := range invalid {
		t.Run(strconv.Itoa(zIndex), func(t *testing.T) {
			element := NewTextElement("invalid", "Invalid", FontNormal)
			element.ZIndex = &zIndex
			if err := NewDisplayElements("app", element).Validate(); err == nil {
				t.Fatalf("Validate accepted z_index %d", zIndex)
			}
		})
	}
}

func TestXPMBitmapElementValidationBoundaries(t *testing.T) {
	zero := 0
	hundred := 100
	for _, element := range []XPMBitmapElement{
		{BaseDisplayElement: BaseDisplayElement{ID: "zero"}, Data: "xpm", Opacity: &zero},
		{BaseDisplayElement: BaseDisplayElement{ID: "hundred"}, Data: "xpm", Opacity: &hundred},
	} {
		if err := NewDisplayElements("app", element).Validate(); err != nil {
			t.Fatalf("Validate opacity %d: %v", *element.Opacity, err)
		}
	}

	belowMinimum := -1
	aboveMaximum := 101
	invalid := []XPMBitmapElement{
		{BaseDisplayElement: BaseDisplayElement{ID: "empty"}},
		{BaseDisplayElement: BaseDisplayElement{ID: "below"}, Data: "xpm", Opacity: &belowMinimum},
		{BaseDisplayElement: BaseDisplayElement{ID: "above"}, Data: "xpm", Opacity: &aboveMaximum},
	}
	for _, element := range invalid {
		t.Run(element.ID, func(t *testing.T) {
			if err := NewDisplayElements("app", element).Validate(); err == nil {
				t.Fatal("Validate accepted invalid XPM bitmap element")
			}
		})
	}

	invalidPointers := []*XPMBitmapElement{
		{BaseDisplayElement: BaseDisplayElement{ID: "pointer-empty"}},
		{BaseDisplayElement: BaseDisplayElement{ID: "pointer-opacity"}, Data: "xpm", Opacity: &aboveMaximum},
	}
	for _, element := range invalidPointers {
		t.Run(element.ID, func(t *testing.T) {
			if err := NewDisplayElements("app", element).Validate(); err == nil {
				t.Fatal("Validate accepted invalid XPM bitmap pointer")
			}
		})
	}

	var nilElement *XPMBitmapElement
	if err := NewDisplayElements("app", nilElement).Validate(); err == nil {
		t.Fatal("Validate accepted typed-nil XPM bitmap pointer")
	}
}

func TestClearDisplayElementsSendsIDsOnlyInJSONBody(t *testing.T) {
	maximumApplicationName := strings.Repeat("a", 31)
	cases := []serviceRequestCase{
		successJSONCase(
			"application query and all valid ID punctuation",
			http.MethodDelete,
			"/api/display/draw",
			"application_name=my_app",
			`{"element_ids":["title","image.1","hero_2","section-3"]}`,
			func(ctx context.Context, client *Client) error {
				return client.Display().ClearElements(ctx, ClearDisplayElementsRequest{
					ApplicationName: "my_app",
					ElementIDs:      []string{"title", "image.1", "hero_2", "section-3"},
				})
			},
			`{"result":"OK"}`,
		),
		successJSONCase(
			"empty application and one uppercase ID",
			http.MethodDelete,
			"/api/display/draw",
			"",
			`{"element_ids":["UPPERCASE"]}`,
			func(ctx context.Context, client *Client) error {
				return client.Display().ClearElements(ctx, ClearDisplayElementsRequest{ElementIDs: []string{"UPPERCASE"}})
			},
			`{"result":"OK"}`,
		),
		successJSONCase(
			"maximum application name length",
			http.MethodDelete,
			"/api/display/draw",
			"application_name="+maximumApplicationName,
			`{"element_ids":["element"]}`,
			func(ctx context.Context, client *Client) error {
				return client.Display().ClearElements(ctx, ClearDisplayElementsRequest{
					ApplicationName: maximumApplicationName,
					ElementIDs:      []string{"element"},
				})
			},
			`{"result":"OK"}`,
		),
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requestErr := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestErr <- serviceRequestError(tc, r)
				writeJSON(t, w, map[string]string{"result": "OK"})
			}))
			defer server.Close()

			client, err := NewClient(
				WithBaseURL(server.URL),
				WithVersionNegotiation(VersionNegotiationDisabled),
				WithHTTPClient(server.Client()),
			)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			if err := tc.call(context.Background(), client); err != nil {
				t.Fatalf("ClearElements: %v", err)
			}
			if err := <-requestErr; err != nil {
				t.Fatalf("request contract: %v", err)
			}
		})
	}
}

func TestClearDisplayElementsRejectsInvalidRequestsBeforeTransport(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeJSON(t, w, map[string]string{"result": "OK"})
	}))
	defer server.Close()

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	requestsToReject := []struct {
		name    string
		request ClearDisplayElementsRequest
	}{
		{name: "nil IDs", request: ClearDisplayElementsRequest{ElementIDs: nil}},
		{name: "empty IDs", request: ClearDisplayElementsRequest{ElementIDs: []string{}}},
		{name: "empty ID", request: ClearDisplayElementsRequest{ElementIDs: []string{""}}},
		{name: "slash", request: ClearDisplayElementsRequest{ElementIDs: []string{"valid", "invalid/id"}}},
		{name: "space", request: ClearDisplayElementsRequest{ElementIDs: []string{"invalid id"}}},
		{name: "non-ASCII", request: ClearDisplayElementsRequest{ElementIDs: []string{"element-ñ"}}},
		{
			name: "application name too long",
			request: ClearDisplayElementsRequest{
				ApplicationName: strings.Repeat("a", 32),
				ElementIDs:      []string{"valid"},
			},
		},
	}
	for _, tc := range requestsToReject {
		t.Run(tc.name, func(t *testing.T) {
			err := client.Display().ClearElements(context.Background(), tc.request)
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("ClearElements error = %T %v, want ValidationError", err, err)
			}
		})
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("server requests = %d, want 0", got)
	}
}
