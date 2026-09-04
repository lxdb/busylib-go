package busylib

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
)

// DisplayService controls brightness and rendered display content.
type DisplayService struct {
	client *Client
}

// Display returns the display control API.
func (c *Client) Display() DisplayService { return DisplayService{client: c} }

// DisplayBrightnessInfo contains "auto" or a percentage string from 0 through
// 100.
type DisplayBrightnessInfo struct {
	Value string `json:"value"`
}

// DisplayElements defines one application display update. Elements share the
// application name and priority; each element selects its own physical display.
type DisplayElements struct {
	ApplicationName      string           `json:"application_name"`
	Priority             int              `json:"priority,omitempty"`
	LEDNotificationColor string           `json:"led_notification_color,omitempty"`
	Elements             []DisplayElement `json:"elements"`
}

// ClearDisplayElementsRequest selects rendered elements to remove.
type ClearDisplayElementsRequest struct {
	// ApplicationName limits removal to one application. An empty value applies
	// the ElementIDs filter across all applications.
	ApplicationName string `json:"-"`
	// ElementIDs contains one or more rendered element identifiers.
	ElementIDs []string `json:"element_ids"`
}

// DefaultDisplayPriority is the priority assigned by NewDisplayElements.
const DefaultDisplayPriority = 50

// NewDisplayElements creates a front-display request with the default priority.
func NewDisplayElements(applicationName string, elements ...DisplayElement) DisplayElements {
	return DisplayElements{
		ApplicationName: applicationName,
		Priority:        DefaultDisplayPriority,
		Elements:        elements,
	}
}

// Brightness returns the current brightness value or automatic mode.
func (s DisplayService) Brightness(ctx context.Context) (DisplayBrightnessInfo, error) {
	var out DisplayBrightnessInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/display/brightness", nil, nil, &out)
	return out, err
}

// SetBrightness selects automatic mode or a percentage from 0 through 100.
func (s DisplayService) SetBrightness(ctx context.Context, value string) error {
	if err := validateBrightness(value); err != nil {
		return validationError(http.MethodPost, "/api/display/brightness", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/display/brightness", url.Values{"value": []string{value}}, nil)
}

// Draw validates and renders application elements on the device displays.
func (s DisplayService) Draw(ctx context.Context, request DisplayElements) error {
	if err := request.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/display/draw", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/display/draw", nil, JSONBody(request))
}

// Clear removes rendered elements for one application.
// An empty application name removes all rendered elements.
func (s DisplayService) Clear(ctx context.Context, applicationName string) error {
	if applicationName != "" {
		if err := validateApplicationName(applicationName); err != nil {
			return validationError(http.MethodDelete, "/api/display/draw", err.Error(), err)
		}
	}
	query := url.Values{}
	if applicationName != "" {
		query.Set("application_name", applicationName)
	}
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/display/draw", query, nil)
}

// ClearElements removes selected rendered elements. A non-empty ApplicationName
// limits removal to that application. An empty ApplicationName applies the
// ElementIDs filter across all applications. ApplicationName is sent as a query
// parameter because firmware 1.2.3 ignores it in the request body.
//
// WARNING: Firmware 1.2.3 has an unterminated internal element_ids pointer
// array. This operation may behave incorrectly or restart the device.
func (s DisplayService) ClearElements(ctx context.Context, request ClearDisplayElementsRequest) error {
	if err := validateClearDisplayElementsRequest(request); err != nil {
		return validationError(http.MethodDelete, "/api/display/draw", err.Error(), err)
	}
	query := url.Values{}
	if request.ApplicationName != "" {
		query.Set("application_name", request.ApplicationName)
	}
	return s.client.doSuccess(ctx, http.MethodDelete, "/api/display/draw", query, JSONBody(request))
}

// Screen returns the uncompressed pixels of the selected display. Pass the
// result to frame.FromHTTP when metadata or image conversion is required.
func (s DisplayService) Screen(ctx context.Context, display DisplayTarget) ([]byte, error) {
	if err := validateScreenDisplay(display); err != nil {
		return nil, validationError(http.MethodGet, "/api/screen", err.Error(), err)
	}
	displayNumber := 0
	if display == DisplayBack {
		displayNumber = 1
	}
	response, err := s.client.Do(ctx, Request{
		Method:       http.MethodGet,
		Path:         "/api/screen",
		Query:        url.Values{"display": []string{strconv.Itoa(displayNumber)}},
		ResponseMode: ResponseModeBytes,
	})
	if err != nil {
		return nil, err
	}
	decoded, err := base64.StdEncoding.DecodeString(string(response.Body))
	if err != nil {
		return nil, &ProtocolError{
			Method:    http.MethodGet,
			Path:      "/api/screen",
			RequestID: response.RequestID,
			Excerpt:   excerpt(response.Body),
			Err:       err,
		}
	}
	return decoded, nil
}
