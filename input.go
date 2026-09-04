package busylib

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// InputService sends virtual key input to the device.
type InputService struct {
	client *Client
}

// Input returns the virtual input API.
func (c *Client) Input() InputService { return InputService{client: c} }

// InputKey identifies a physical or virtual device key.
type InputKey string

const (
	// InputKeyUp sends the Up key.
	InputKeyUp InputKey = "up"
	// InputKeyDown sends the Down key.
	InputKeyDown InputKey = "down"
	// InputKeyOK sends the OK key.
	InputKeyOK InputKey = "ok"
	// InputKeyBack sends the Back key.
	InputKeyBack InputKey = "back"
	// InputKeyStart sends the Start key.
	InputKeyStart InputKey = "start"
	// InputKeyBusy sends the Busy key.
	InputKeyBusy InputKey = "busy"
	// InputKeyCustom sends the Custom key.
	InputKeyCustom InputKey = "custom"
	// InputKeyOff sends the Off key.
	InputKeyOff InputKey = "off"
	// InputKeyApps sends the Apps key.
	InputKeyApps InputKey = "apps"
	// InputKeySettings sends the Settings key.
	InputKeySettings InputKey = "settings"
)

// SendKey sends one virtual key press to the device.
func (s InputService) SendKey(ctx context.Context, key InputKey) error {
	if err := validateInputKey(key); err != nil {
		return validationError(http.MethodPost, "/api/input", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/input", url.Values{"key": []string{string(key)}}, nil)
}

func validateInputKey(key InputKey) error {
	switch key {
	case InputKeyUp, InputKeyDown, InputKeyOK, InputKeyBack, InputKeyStart, InputKeyBusy, InputKeyCustom, InputKeyOff, InputKeyApps, InputKeySettings:
		return nil
	default:
		return fmt.Errorf("key %q is not supported", key)
	}
}
