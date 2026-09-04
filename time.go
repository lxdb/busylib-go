package busylib

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TimeService reads and changes the device clock and timezone.
type TimeService struct {
	client *Client
}

// Time returns the device clock API.
func (c *Client) Time() TimeService { return TimeService{client: c} }

// TimestampInfo contains an RFC 3339 device timestamp.
type TimestampInfo struct {
	Timestamp string `json:"timestamp"`
}

// TimezoneInfo describes a firmware-supported time zone.
type TimezoneInfo struct {
	Name   string `json:"name"`
	Offset string `json:"offset"`
	Abbr   string `json:"abbr"`
}

// TimezoneListResponse contains the firmware-supported time zones.
type TimezoneListResponse struct {
	List []TimezoneInfo `json:"list"`
}

// Now returns the device timestamp.
func (s TimeService) Now(ctx context.Context) (TimestampInfo, error) {
	var out TimestampInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/time", nil, nil, &out)
	return out, err
}

// SetTimestamp changes the device clock from an RFC 3339 timestamp.
func (s TimeService) SetTimestamp(ctx context.Context, timestamp string) error {
	if err := validateTimestamp(timestamp); err != nil {
		return validationError(http.MethodPost, "/api/time/timestamp", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/time/timestamp", url.Values{"timestamp": []string{timestamp}}, nil)
}

// Timezone returns the current device timezone.
func (s TimeService) Timezone(ctx context.Context) (TimezoneInfo, error) {
	var out TimezoneInfo
	err := s.client.doJSON(ctx, http.MethodGet, "/api/time/timezone", nil, nil, &out)
	return out, err
}

// SetTimezone changes the device timezone.
func (s TimeService) SetTimezone(ctx context.Context, timezone string) error {
	if err := validateTimezone(timezone); err != nil {
		return validationError(http.MethodPost, "/api/time/timezone", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/time/timezone", url.Values{"timezone": []string{timezone}}, nil)
}

// Timezones returns the timezone names supported by the device.
func (s TimeService) Timezones(ctx context.Context) (TimezoneListResponse, error) {
	var out TimezoneListResponse
	err := s.client.doJSON(ctx, http.MethodGet, "/api/time/tzlist", nil, nil, &out)
	return out, err
}

func validateTimestamp(value string) error {
	if len(value) == len("20060102T150405Z") {
		if _, err := time.Parse("20060102T150405Z", value); err == nil {
			return nil
		}
	}
	if len(value) == len("2006-01-02T15:04:05Z") || len(value) == len("2006-01-02T15:04:05+03:00") {
		if _, err := time.Parse(time.RFC3339, value); err == nil {
			return nil
		}
	}
	return errors.New("timestamp must use RFC 3339 or firmware compact UTC format YYYYMMDDThhmmssZ")
}

func validateTimezone(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("timezone must not be empty")
	}
	return nil
}
