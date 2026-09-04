package busylib

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
)

const (
	maxBusyTitleBytes = 128
	maxBusyThemeBytes = 64

	busyTimerSimpleMaxMS   int64 = 24 * 60 * 60 * 1000
	busyTimerIntervalMinMS int64 = 5 * 60 * 1000
	busyTimerIntervalMaxMS int64 = 8 * 60 * 60 * 1000
	busyTimerCyclesMin           = 2
	busyTimerCyclesMax           = 35
)

// BusyService reads and changes busy-state snapshots and profiles.
type BusyService struct {
	client *Client
}

// Busy returns the busy-state API.
func (c *Client) Busy() BusyService { return BusyService{client: c} }

// BusySnapshot contains the complete current timer state and its device
// timestamp in milliseconds.
type BusySnapshot struct {
	Snapshot            BusySnapshotData `json:"snapshot"`
	SnapshotTimestampMS int64            `json:"snapshot_timestamp_ms"`
}

// BusySnapshotData describes the active timer and Busy Bar settings. Required
// pointer fields depend on Type; Validate checks the complete combination.
type BusySnapshotData struct {
	Type                       BusySnapshotType           `json:"type"`
	CardID                     string                     `json:"card_id,omitempty"`
	TimeLeftMS                 *int64                     `json:"time_left_ms,omitempty"`
	IsPaused                   *bool                      `json:"is_paused,omitempty"`
	CurrentInterval            *int                       `json:"current_interval,omitempty"`
	CurrentIntervalTimeTotalMS *int64                     `json:"current_interval_time_total_ms,omitempty"`
	CurrentIntervalTimeLeftMS  *int64                     `json:"current_interval_time_left_ms,omitempty"`
	IntervalSettings           *BusyTimerIntervalSettings `json:"interval_settings,omitempty"`
	BusyBarSettings            BusyBarSettings            `json:"busy_bar_settings"`
}

// BusySnapshotType identifies the active timer mode.
type BusySnapshotType string

const (
	// BusySnapshotNotStarted means no timer has started.
	BusySnapshotNotStarted BusySnapshotType = "NOT_STARTED"
	// BusySnapshotInfinite means an infinite timer is active.
	BusySnapshotInfinite BusySnapshotType = "INFINITE"
	// BusySnapshotSimple means a fixed-duration timer is active.
	BusySnapshotSimple BusySnapshotType = "SIMPLE"
	// BusySnapshotInterval means an interval timer is active.
	BusySnapshotInterval BusySnapshotType = "INTERVAL"
)

// BusyProfile defines the complete contents of a saved timer profile.
type BusyProfile struct {
	SortOrder          int               `json:"sort_order"`
	Title              string            `json:"title"`
	ID                 string            `json:"id"`
	TimerSettings      BusyTimerSettings `json:"timer_settings"`
	BusyBarSettings    BusyBarSettings   `json:"busy_bar_settings"`
	ProfileTimestampMS int64             `json:"profile_timestamp_ms"`
}

// BusyProfileSlot selects a built-in profile slot.
type BusyProfileSlot string

const (
	// BusyProfileSlotBusy selects the Busy button profile.
	BusyProfileSlotBusy BusyProfileSlot = "busy"
	// BusyProfileSlotCustom selects the Custom button profile.
	BusyProfileSlotCustom BusyProfileSlot = "custom"
)

// BusyTimerSettings configures an infinite, simple, or interval timer. Required
// pointer fields depend on Type; duration fields use milliseconds.
type BusyTimerSettings struct {
	Type                    BusyTimerType `json:"type"`
	TotalTimeMS             *int64        `json:"total_time_ms,omitempty"`
	IntervalWorkMS          *int64        `json:"interval_work_ms,omitempty"`
	IntervalRestMS          *int64        `json:"interval_rest_ms,omitempty"`
	IntervalWorkCyclesCount *int          `json:"interval_work_cycles_count,omitempty"`
	IsAutostartEnabled      *bool         `json:"is_autostart_enabled,omitempty"`
}

// BusyTimerType identifies a timer mode.
type BusyTimerType string

const (
	// BusyTimerInfinite runs until the user stops it.
	BusyTimerInfinite BusyTimerType = "INFINITE"
	// BusyTimerSimple runs for one fixed duration.
	BusyTimerSimple BusyTimerType = "SIMPLE"
	// BusyTimerInterval alternates work and rest periods.
	BusyTimerInterval BusyTimerType = "INTERVAL"
)

// BusyTimerIntervalSettings describes the active interval timer.
type BusyTimerIntervalSettings struct {
	Type                    BusyTimerType `json:"type"`
	IntervalWorkMS          int64         `json:"interval_work_ms"`
	IntervalRestMS          int64         `json:"interval_rest_ms"`
	IntervalWorkCyclesCount int           `json:"interval_work_cycles_count"`
	IsAutostartEnabled      bool          `json:"is_autostart_enabled"`
}

// BusyBarSettings controls timer display and smart-home behavior.
type BusyBarSettings struct {
	Theme             string `json:"theme"`
	ShowWorkPhaseOnly bool   `json:"show_work_phase_only"`
	TriggerSmartHome  bool   `json:"trigger_smart_home"`
}

// Snapshot returns the current busy-state snapshot.
func (s BusyService) Snapshot(ctx context.Context) (BusySnapshot, error) {
	var out BusySnapshot
	err := s.client.doJSON(ctx, http.MethodGet, "/api/busy/snapshot", nil, nil, &out)
	return out, err
}

// SetSnapshot validates and replaces the complete current busy-state snapshot.
// Read Snapshot first when fields not owned by the caller must be preserved.
func (s BusyService) SetSnapshot(ctx context.Context, snapshot BusySnapshot) error {
	if err := snapshot.Validate(); err != nil {
		return validationError(http.MethodPut, "/api/busy/snapshot", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPut, "/api/busy/snapshot", nil, JSONBody(snapshot))
}

// Profile returns the saved busy profile in slot.
func (s BusyService) Profile(ctx context.Context, slot BusyProfileSlot) (BusyProfile, error) {
	var out BusyProfile
	if err := validateBusyProfileSlot(slot); err != nil {
		return out, validationError(http.MethodGet, "/api/busy/profiles/{slot}", err.Error(), err)
	}
	err := s.client.doJSON(ctx, http.MethodGet, "/api/busy/profiles/"+url.PathEscape(string(slot)), nil, nil, &out)
	return out, err
}

// SetProfile validates and replaces the complete saved busy profile in slot.
func (s BusyService) SetProfile(ctx context.Context, slot BusyProfileSlot, profile BusyProfile) error {
	if err := validateBusyProfileSlot(slot); err != nil {
		return validationError(http.MethodPut, "/api/busy/profiles/{slot}", err.Error(), err)
	}
	if err := profile.Validate(); err != nil {
		return validationError(http.MethodPut, "/api/busy/profiles/{slot}", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPut, "/api/busy/profiles/"+url.PathEscape(string(slot)), nil, JSONBody(profile))
}

// Validate reports whether a busy-state snapshot meets the device contract.
func (snapshot BusySnapshot) Validate() error {
	if err := snapshot.Snapshot.BusyBarSettings.Validate(); err != nil {
		return fieldError("snapshot.busy_bar_settings", err)
	}

	data := snapshot.Snapshot
	switch data.Type {
	case BusySnapshotNotStarted:
		return nil
	case BusySnapshotInfinite:
		if err := validateBusyCardID("snapshot.card_id", data.CardID); err != nil {
			return err
		}
		if data.IsPaused == nil {
			return errors.New("snapshot.is_paused is required for INFINITE snapshots")
		}
		return nil
	case BusySnapshotSimple:
		if err := validateBusyCardID("snapshot.card_id", data.CardID); err != nil {
			return err
		}
		if data.IsPaused == nil || data.TimeLeftMS == nil {
			return errors.New("snapshot.is_paused and snapshot.time_left_ms are required for SIMPLE snapshots")
		}
		return validateBusyDuration("snapshot.time_left_ms", *data.TimeLeftMS, 0, busyTimerSimpleMaxMS)
	case BusySnapshotInterval:
		if err := validateBusyCardID("snapshot.card_id", data.CardID); err != nil {
			return err
		}
		if data.IsPaused == nil || data.CurrentInterval == nil ||
			data.CurrentIntervalTimeTotalMS == nil || data.CurrentIntervalTimeLeftMS == nil ||
			data.IntervalSettings == nil {
			return errors.New("snapshot.is_paused, current interval fields, and interval_settings are required for INTERVAL snapshots")
		}
		if err := data.IntervalSettings.Validate(); err != nil {
			return fieldError("snapshot.interval_settings", err)
		}
		if *data.CurrentInterval < 0 || *data.CurrentInterval > (busyTimerCyclesMax*2)-2 {
			return fmt.Errorf("snapshot.current_interval must be between 0 and %d", (busyTimerCyclesMax*2)-2)
		}
		total := *data.CurrentIntervalTimeTotalMS
		left := *data.CurrentIntervalTimeLeftMS
		if total < 0 || total > math.MaxUint32 || left < 0 || left > total {
			return errors.New("snapshot interval times must fit unsigned 32-bit values and time left must not exceed total")
		}
		upperBound := data.IntervalSettings.IntervalWorkMS
		if *data.CurrentInterval%2 == 1 {
			upperBound = data.IntervalSettings.IntervalRestMS
		}
		if left > upperBound {
			return errors.New("snapshot.current_interval_time_left_ms exceeds the configured interval duration")
		}
		return nil
	default:
		return fmt.Errorf("snapshot.type %q is not supported", data.Type)
	}
}

// Validate reports whether a busy profile has supported display and timer settings.
func (profile BusyProfile) Validate() error {
	if profile.SortOrder < math.MinInt32 || profile.SortOrder > math.MaxInt32 {
		return errors.New("sort_order must fit a signed 32-bit integer")
	}
	if len(profile.Title) > maxBusyTitleBytes {
		return fmt.Errorf("title must be at most %d bytes", maxBusyTitleBytes)
	}
	if err := validateBusyCardID("id", profile.ID); err != nil {
		return err
	}
	if err := profile.TimerSettings.Validate(); err != nil {
		return fieldError("timer_settings", err)
	}
	return profile.BusyBarSettings.Validate()
}

// Validate reports whether busy timer settings meet their selected timer type.
func (settings BusyTimerSettings) Validate() error {
	switch settings.Type {
	case BusyTimerInfinite:
		return nil
	case BusyTimerSimple:
		if settings.TotalTimeMS == nil {
			return errors.New("total_time_ms is required for SIMPLE timers")
		}
		return validateBusyDuration("total_time_ms", *settings.TotalTimeMS, 0, busyTimerSimpleMaxMS)
	case BusyTimerInterval:
		if settings.IntervalWorkMS == nil || settings.IntervalRestMS == nil ||
			settings.IntervalWorkCyclesCount == nil || settings.IsAutostartEnabled == nil {
			return errors.New("interval work, rest, cycle count, and autostart fields are required for INTERVAL timers")
		}
		return validateBusyInterval(
			*settings.IntervalWorkMS,
			*settings.IntervalRestMS,
			*settings.IntervalWorkCyclesCount,
		)
	default:
		return fmt.Errorf("type %q is not supported", settings.Type)
	}
}

// Validate reports whether interval durations and cycles meet device limits.
func (settings BusyTimerIntervalSettings) Validate() error {
	return validateBusyInterval(
		settings.IntervalWorkMS,
		settings.IntervalRestMS,
		settings.IntervalWorkCyclesCount,
	)
}

// Validate reports whether busy-bar light settings use valid colors and effects.
func (settings BusyBarSettings) Validate() error {
	if len(settings.Theme) > maxBusyThemeBytes {
		return fmt.Errorf("theme must be at most %d bytes", maxBusyThemeBytes)
	}
	return nil
}

func validateBusyInterval(workMS, restMS int64, cycles int) error {
	if err := validateBusyDuration("interval_work_ms", workMS, busyTimerIntervalMinMS, busyTimerIntervalMaxMS); err != nil {
		return err
	}
	if err := validateBusyDuration("interval_rest_ms", restMS, busyTimerIntervalMinMS, busyTimerIntervalMaxMS); err != nil {
		return err
	}
	if cycles < busyTimerCyclesMin || cycles > busyTimerCyclesMax {
		return fmt.Errorf("interval_work_cycles_count must be between %d and %d", busyTimerCyclesMin, busyTimerCyclesMax)
	}
	return nil
}

func validateBusyDuration(field string, value, minimum, maximum int64) error {
	if value < minimum || value > maximum || value > math.MaxUint32 {
		return fmt.Errorf("%s must be between %d and %d milliseconds", field, minimum, maximum)
	}
	return nil
}

func validateBusyCardID(field, value string) error {
	if len(value) != 36 {
		return fmt.Errorf("%s must be a 36-character UUID", field)
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return fmt.Errorf("%s must be a UUID", field)
			}
			continue
		}
		if (character < '0' || character > '9') &&
			(character < 'a' || character > 'f') &&
			(character < 'A' || character > 'F') {
			return fmt.Errorf("%s must be a UUID", field)
		}
	}
	return nil
}

func validateBusyProfileSlot(slot BusyProfileSlot) error {
	if slot != BusyProfileSlotBusy && slot != BusyProfileSlotCustom {
		return fmt.Errorf("profile slot %q is not supported", slot)
	}
	return nil
}
