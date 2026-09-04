package busylib

import (
	"errors"
	"fmt"
	"math"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	colorPattern       = regexp.MustCompile(`^#[a-fA-F0-9]{8}$`)
	httpKeyPattern     = regexp.MustCompile(`^[0-9]{4,10}$`)
	brightnessPattern  = regexp.MustCompile(`^(auto|[0-9]{1,2}|100)$`)
	logFilenamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

const (
	maxDisplayElements       = 100
	maxAssetParameterBytes   = 31
	maxDisplayQueryBytes     = 63
	maxStoragePathBytes      = 63
	maxLogFilenameBytes      = 63
	maxUpdateVersionBytes    = 63
	maxAccountServerURLBytes = 64
	maxWiFiSSIDBytes         = 33
	maxWiFiPasswordBytes     = 63
	maxBusyTitleBytes        = 128
	maxBusyThemeBytes        = 64

	busyTimerSimpleMaxMS   int64 = 24 * 60 * 60 * 1000
	busyTimerIntervalMinMS int64 = 5 * 60 * 1000
	busyTimerIntervalMaxMS int64 = 8 * 60 * 60 * 1000
	busyTimerCyclesMin           = 2
	busyTimerCyclesMax           = 35
)

// ValidationWarning describes a product-contract concern that is useful to
// callers but does not make the request invalid for the device API.
type ValidationWarning struct {
	Field   string
	Message string
}

// NormalizeColor accepts exactly #RRGGBBAA and returns the value with uppercase
// hexadecimal digits.
func NormalizeColor(value string) (string, error) {
	if !colorPattern.MatchString(value) {
		return "", fmt.Errorf("color must use #RRGGBBAA")
	}
	return strings.ToUpper(value), nil
}

// Validate reports whether a display request meets the locally recorded device
// API contract. It does not contact the device.
func (request DisplayElements) Validate() error {
	if request.Priority < 0 || request.Priority > 100 {
		return errors.New("priority must be omitted or between 1 and 100")
	}
	if request.LEDNotificationColor != "" {
		if _, err := NormalizeColor(request.LEDNotificationColor); err != nil {
			return fieldError("led_notification_color", err)
		}
	}
	if len(request.Elements) == 0 {
		return errors.New("elements must contain at least one element")
	}
	if len(request.Elements) > maxDisplayElements {
		return fmt.Errorf("elements must contain at most %d elements", maxDisplayElements)
	}
	for index, element := range request.Elements {
		if element == nil {
			return fmt.Errorf("elements[%d] must not be nil", index)
		}
		if err := validateDisplayElement(index, request.ApplicationName, element); err != nil {
			return err
		}
	}
	return nil
}

// Warnings reports nonfatal display concerns, such as coordinates outside the
// observed screen bounds. It returns nil when no concerns exist.
func (request DisplayElements) Warnings() []ValidationWarning {
	var warnings []ValidationWarning
	for index, element := range request.Elements {
		base, _, err := displayElementInfo(index, request.ApplicationName, element)
		if err != nil {
			continue
		}
		width, height := 72, 16
		if base.Display == DisplayBack {
			width, height = 160, 80
		}
		if base.X != nil && (*base.X < 0 || *base.X >= width) {
			warnings = append(warnings, ValidationWarning{
				Field:   fmt.Sprintf("elements[%d].x", index),
				Message: fmt.Sprintf("coordinate is outside the observed %dx%d display surface", width, height),
			})
		}
		if base.Y != nil && (*base.Y < 0 || *base.Y >= height) {
			warnings = append(warnings, ValidationWarning{
				Field:   fmt.Sprintf("elements[%d].y", index),
				Message: fmt.Sprintf("coordinate is outside the observed %dx%d display surface", width, height),
			})
		}
	}
	return warnings
}

// Validate reports whether an audio request selects one valid asset source.
func (request PlayAudio) Validate() error {
	return validateAssetSource("audio", request.ApplicationName, request.Path, request.StockPath)
}

// Validate reports whether an asset upload has safe names and a body.
func (request UploadAssetRequest) Validate() error {
	if err := validateAssetParameter("application_name", request.ApplicationName); err != nil {
		return err
	}
	if err := validateAssetParameter("file", request.File); err != nil {
		return err
	}
	if !firmwarePathIsSane("/ext/user_assets/" + request.ApplicationName + "/" + request.File) {
		return errors.New("application_name and file produce an unsafe firmware path")
	}
	if request.Body == nil {
		return errors.New("asset upload body must not be nil")
	}
	return nil
}

// Validate reports whether a storage write has a safe path and a body.
func (request WriteStorageFileRequest) Validate() error {
	if err := validateStoragePath("path", request.Path); err != nil {
		return err
	}
	if request.Body == nil {
		return errors.New("storage write body must not be nil")
	}
	return nil
}

// Validate reports whether Wi-Fi connection settings meet the device contract.
func (request WiFiConnectRequest) Validate() error {
	if request.SSID == "" {
		return errors.New("ssid must not be empty")
	}
	if len(request.SSID) > maxWiFiSSIDBytes {
		return fmt.Errorf("ssid must be at most %d bytes", maxWiFiSSIDBytes)
	}
	if len(request.Password) > maxWiFiPasswordBytes {
		return fmt.Errorf("password must be at most %d bytes", maxWiFiPasswordBytes)
	}
	if !validWiFiConnectSecurity(request.Security) {
		return fmt.Errorf("security %q is not supported", request.Security)
	}
	config := request.IPConfig
	if !validWiFiIPMethod(config.IPMethod) {
		return fmt.Errorf("ip_config.ip_method %q is not supported", config.IPMethod)
	}
	if config.IPMethod == WiFiIPMethodStatic {
		if config.Address == "" || config.Mask == "" || config.Gateway == "" {
			return errors.New("static ip_config requires address, mask, and gateway")
		}
	}
	for field, value := range map[string]string{
		"address": config.Address,
		"mask":    config.Mask,
		"gateway": config.Gateway,
	} {
		if value == "" {
			continue
		}
		address, err := netip.ParseAddr(value)
		if err != nil || !address.Is4() {
			return fmt.Errorf("ip_config.%s must be an IPv4 address", field)
		}
	}
	return nil
}

// Validate reports whether remote account backend settings are syntactically
// valid for the device API.
func (backend AccountBackend) Validate() error {
	if strings.TrimSpace(backend.ServerURL) == "" {
		return errors.New("server_url must not be empty")
	}
	if len(backend.ServerURL) > maxAccountServerURLBytes {
		return fmt.Errorf("server_url must be at most %d bytes", maxAccountServerURLBytes)
	}
	if backend.ServerURL != "default" &&
		!strings.HasPrefix(backend.ServerURL, "mqtt://") &&
		!strings.HasPrefix(backend.ServerURL, "mqtts://") {
		return errors.New("server_url must be default, mqtt://, or mqtts://")
	}
	if !validAccountClientCertType(backend.ClientCertType) {
		return fmt.Errorf("client_cert_type %q is not supported", backend.ClientCertType)
	}
	return nil
}

// Validate reports whether a smart-home switch update has supported values.
func (update SmartHomeSwitchUpdate) Validate() error {
	if update.State == nil && update.Startup == "" {
		return errors.New("state or startup must be provided")
	}
	if update.Startup != "" && !validSmartHomeSwitchStartup(update.Startup) {
		return fmt.Errorf("startup %q is not supported", update.Startup)
	}
	return nil
}

// Validate reports whether automatic update settings use a supported schedule.
func (settings AutoUpdateSettings) Validate() error {
	if settings.IntervalStart != "" && !validFirmwareClock(settings.IntervalStart) {
		return errors.New("interval_start must contain an hour from 0-23 and minute from 0-59")
	}
	if settings.IntervalEnd != "" && !validFirmwareClock(settings.IntervalEnd) {
		return errors.New("interval_end must contain an hour from 0-23 and minute from 0-59")
	}
	return nil
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

func validateDisplayElement(index int, applicationName string, element DisplayElement) error {
	base, validateSpecific, err := displayElementInfo(index, applicationName, element)
	if err != nil {
		return err
	}
	if err := validateBaseDisplayElement(index, base); err != nil {
		return err
	}
	return validateSpecific(index)
}

func displayElementInfo(index int, applicationName string, element DisplayElement) (BaseDisplayElement, func(int) error, error) {
	switch value := element.(type) {
	case TextElement:
		return value.BaseDisplayElement, func(index int) error { return validateTextElement(index, value) }, nil
	case *TextElement:
		if value == nil {
			return BaseDisplayElement{}, nil, fmt.Errorf("elements[%d] must not be nil", index)
		}
		return value.BaseDisplayElement, func(index int) error { return validateTextElement(index, *value) }, nil
	case ImageElement:
		return value.BaseDisplayElement, func(index int) error {
			if err := validateAssetSource(fmt.Sprintf("elements[%d]", index), applicationName, value.Path, value.StockPath); err != nil {
				return err
			}
			return validateOptionalPercent(fmt.Sprintf("elements[%d].opacity", index), value.Opacity)
		}, nil
	case *ImageElement:
		if value == nil {
			return BaseDisplayElement{}, nil, fmt.Errorf("elements[%d] must not be nil", index)
		}
		return value.BaseDisplayElement, func(index int) error {
			if err := validateAssetSource(fmt.Sprintf("elements[%d]", index), applicationName, value.Path, value.StockPath); err != nil {
				return err
			}
			return validateOptionalPercent(fmt.Sprintf("elements[%d].opacity", index), value.Opacity)
		}, nil
	case AnimationElement:
		return value.BaseDisplayElement, func(index int) error {
			if err := validateAssetSource(fmt.Sprintf("elements[%d]", index), applicationName, value.Path, value.StockPath); err != nil {
				return err
			}
			return validateOptionalPercent(fmt.Sprintf("elements[%d].opacity", index), value.Opacity)
		}, nil
	case *AnimationElement:
		if value == nil {
			return BaseDisplayElement{}, nil, fmt.Errorf("elements[%d] must not be nil", index)
		}
		return value.BaseDisplayElement, func(index int) error {
			if err := validateAssetSource(fmt.Sprintf("elements[%d]", index), applicationName, value.Path, value.StockPath); err != nil {
				return err
			}
			return validateOptionalPercent(fmt.Sprintf("elements[%d].opacity", index), value.Opacity)
		}, nil
	case CountdownElement:
		return value.BaseDisplayElement, func(index int) error { return validateCountdownElement(index, value) }, nil
	case *CountdownElement:
		if value == nil {
			return BaseDisplayElement{}, nil, fmt.Errorf("elements[%d] must not be nil", index)
		}
		return value.BaseDisplayElement, func(index int) error { return validateCountdownElement(index, *value) }, nil
	case RectangleElement:
		return value.BaseDisplayElement, func(index int) error { return validateRectangleElement(index, value) }, nil
	case *RectangleElement:
		if value == nil {
			return BaseDisplayElement{}, nil, fmt.Errorf("elements[%d] must not be nil", index)
		}
		return value.BaseDisplayElement, func(index int) error { return validateRectangleElement(index, *value) }, nil
	default:
		return BaseDisplayElement{}, nil, fmt.Errorf("elements[%d] has unsupported type %T", index, element)
	}
}

func validateBaseDisplayElement(index int, base BaseDisplayElement) error {
	prefix := fmt.Sprintf("elements[%d]", index)
	if base.Timeout != nil && (*base.Timeout < math.MinInt32 || *base.Timeout > math.MaxInt32) {
		return fmt.Errorf("%s.timeout must fit a signed 32-bit integer", prefix)
	}
	displayUntil := int64(0)
	if base.DisplayUntil != "" {
		parsed, err := strconv.ParseInt(base.DisplayUntil, 10, 64)
		if err != nil {
			return fmt.Errorf("%s.display_until must be a signed Unix timestamp string", prefix)
		}
		displayUntil = parsed
	}
	if base.Timeout != nil && *base.Timeout > 0 && displayUntil > 0 {
		return fmt.Errorf("%s.timeout and display_until are mutually exclusive when positive", prefix)
	}
	if base.X != nil && (*base.X < math.MinInt16 || *base.X > math.MaxInt16) {
		return fmt.Errorf("%s.x must fit a signed 16-bit integer", prefix)
	}
	if base.Y != nil && (*base.Y < math.MinInt16 || *base.Y > math.MaxInt16) {
		return fmt.Errorf("%s.y must fit a signed 16-bit integer", prefix)
	}
	if base.Display != "" && base.Display != DisplayFront && base.Display != DisplayBack {
		return fmt.Errorf("%s.display %q is not supported", prefix, base.Display)
	}
	if base.Align != "" && !validDisplayAlign(base.Align) {
		return fmt.Errorf("%s.align %q is not supported", prefix, base.Align)
	}
	return nil
}

func validateTextElement(index int, element TextElement) error {
	prefix := fmt.Sprintf("elements[%d]", index)
	if !validFont(element.Font) {
		return fmt.Errorf("%s.font %q is not supported", prefix, element.Font)
	}
	if element.Color != "" {
		if _, err := NormalizeColor(element.Color); err != nil {
			return fieldError(prefix+".color", err)
		}
	}
	if element.Width < 0 || uint64(element.Width) > math.MaxUint32 {
		return fmt.Errorf("%s.width must be omitted or fit a positive firmware size", prefix)
	}
	if element.ScrollRate < 0 || uint64(element.ScrollRate) > math.MaxUint32 ||
		element.ScrollStartDelay < 0 || uint64(element.ScrollStartDelay) > math.MaxUint32 ||
		element.ScrollRepeatDelay < 0 || uint64(element.ScrollRepeatDelay) > math.MaxUint32 {
		return fmt.Errorf("%s scroll values must fit unsigned firmware sizes", prefix)
	}
	return nil
}

func validateCountdownElement(index int, element CountdownElement) error {
	prefix := fmt.Sprintf("elements[%d]", index)
	if element.Timestamp == "" {
		return fmt.Errorf("%s.timestamp must be a signed Unix timestamp string", prefix)
	}
	if _, err := strconv.ParseInt(element.Timestamp, 10, 64); err != nil {
		return fmt.Errorf("%s.timestamp must be a signed Unix timestamp string", prefix)
	}
	if element.Color != "" {
		if _, err := NormalizeColor(element.Color); err != nil {
			return fieldError(prefix+".color", err)
		}
	}
	if element.Direction != CountdownTimeLeft && element.Direction != CountdownTimeSince {
		return fmt.Errorf("%s.direction %q is not supported", prefix, element.Direction)
	}
	if element.ShowHours != CountdownShowHoursWhenNonZero && element.ShowHours != CountdownShowHoursAlways {
		return fmt.Errorf("%s.show_hours %q is not supported", prefix, element.ShowHours)
	}
	return nil
}

func validateRectangleElement(index int, element RectangleElement) error {
	prefix := fmt.Sprintf("elements[%d]", index)
	if element.Width < 1 || element.Width > math.MaxInt32 {
		return fmt.Errorf("%s.width must be between 1 and %d", prefix, math.MaxInt32)
	}
	if element.Height < 1 || element.Height > math.MaxInt32 {
		return fmt.Errorf("%s.height must be between 1 and %d", prefix, math.MaxInt32)
	}
	if element.Radius < 0 || element.Radius > math.MaxInt32 {
		return fmt.Errorf("%s.radius must be between 0 and %d", prefix, math.MaxInt32)
	}
	if element.Fill != "" && !validRectangleFill(element.Fill) {
		return fmt.Errorf("%s.fill %q is not supported", prefix, element.Fill)
	}
	for colorIndex, color := range element.FillColors {
		if _, err := NormalizeColor(color); err != nil {
			return fieldError(fmt.Sprintf("%s.fill_colors[%d]", prefix, colorIndex), err)
		}
	}
	switch element.Fill {
	case RectangleFillSolid:
		if len(element.FillColors) > 1 {
			return fmt.Errorf("%s.fill_colors must contain at most one color for solid fill", prefix)
		}
	case RectangleFillGradientH, RectangleFillGradientV:
		if len(element.FillColors) != 0 && len(element.FillColors) != 2 {
			return fmt.Errorf("%s.fill_colors must be omitted or contain two colors for gradient fill", prefix)
		}
	default:
		if len(element.FillColors) > 2 {
			return fmt.Errorf("%s.fill_colors must contain at most two colors", prefix)
		}
	}
	if element.BorderWidth != nil && (*element.BorderWidth < 0 || *element.BorderWidth > math.MaxInt32) {
		return fmt.Errorf("%s.border_width must be between 0 and %d", prefix, math.MaxInt32)
	}
	if element.BorderColor != "" {
		if _, err := NormalizeColor(element.BorderColor); err != nil {
			return fieldError(prefix+".border_color", err)
		}
	}
	return nil
}

func validateAssetSource(field, applicationName, path, stockPath string) error {
	hasPath := path != ""
	hasStockPath := stockPath != ""
	if hasPath == hasStockPath {
		return fmt.Errorf("%s must set exactly one of path or stock_path", field)
	}
	if hasPath && !firmwarePathIsSane("/ext/user_assets/"+applicationName+"/"+path) {
		return fmt.Errorf("%s.path produces an unsafe firmware path", field)
	}
	lastSlash := strings.LastIndexByte(stockPath, '/')
	if hasStockPath && (lastSlash < 0 || lastSlash == len(stockPath)-1) {
		return fmt.Errorf("%s.stock_path has invalid stock asset path", field)
	}
	return nil
}

func validateOptionalPercent(field string, value *int) error {
	if value == nil {
		return nil
	}
	if *value < 0 || *value > 100 {
		return fmt.Errorf("%s must be between 0 and 100", field)
	}
	return nil
}

func validateAssetParameter(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	if len(value) > maxAssetParameterBytes {
		return fmt.Errorf("%s must be at most %d bytes", field, maxAssetParameterBytes)
	}
	return nil
}

func validateStoragePath(field, value string) error {
	if value == "" || len(value) > maxStoragePathBytes {
		return fmt.Errorf("%s must be 1-%d bytes", field, maxStoragePathBytes)
	}
	trimmed := strings.TrimSuffix(value, "/")
	if !strings.HasPrefix(trimmed, "/ext") || !firmwarePathIsSane(trimmed) {
		return fmt.Errorf("%s must use a sane firmware path under /ext", field)
	}
	return nil
}

func validateOptionalLogFilename(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > maxLogFilenameBytes || !logFilenamePattern.MatchString(value) {
		return fmt.Errorf("filename must be 1-%d ASCII letters, digits, underscores, or hyphens", maxLogFilenameBytes)
	}
	return nil
}

func firmwarePathIsSane(value string) bool {
	if strings.HasPrefix(value, "~") || strings.HasPrefix(value, "..") {
		return false
	}
	return !strings.Contains(value, "/..") && !strings.Contains(value, `\..`)
}

func validateDeviceName(name string) error {
	if len(name) < 1 || len(name) > 20 {
		return errors.New("device name must be 1-20 characters")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("device name must not contain only spaces")
	}
	const punctuation = " !()_=+;:,.?'|@#$%^&*[]{} /\\\"<>-"
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		if r > 0x7e || r < 0x20 || !strings.ContainsRune(punctuation, r) {
			return errors.New("device name contains unsupported characters")
		}
	}
	return nil
}

func validateHTTPAccess(mode HTTPAccessMode, key string) error {
	if mode != HTTPAccessDisabled && mode != HTTPAccessEnabled && mode != HTTPAccessKey {
		return fmt.Errorf("mode %q is not supported", mode)
	}
	if mode == HTTPAccessKey && !httpKeyPattern.MatchString(key) {
		return errors.New("key mode requires a 4-10 digit key")
	}
	if mode != HTTPAccessKey && key != "" && !httpKeyPattern.MatchString(key) {
		return errors.New("access key must be 4-10 digits when provided")
	}
	return nil
}

func validateBrightness(value string) error {
	if !brightnessPattern.MatchString(value) {
		return errors.New("brightness must be auto or 0-100")
	}
	if value == "auto" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 || parsed > 100 {
		return errors.New("brightness must be auto or 0-100")
	}
	return nil
}

func validateVolume(volume int) error {
	if volume < 0 || volume > 100 {
		return errors.New("volume must be between 0 and 100")
	}
	return nil
}

func validateScreenDisplay(display DisplayTarget) error {
	if display != DisplayFront && display != DisplayBack {
		return errors.New("display must be front or back")
	}
	return nil
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

func validateDisplayApplicationName(value string) error {
	if len(value) > maxDisplayQueryBytes {
		return fmt.Errorf("application_name must be at most %d bytes", maxDisplayQueryBytes)
	}
	return nil
}

func validateBusyProfileSlot(slot BusyProfileSlot) error {
	if slot != BusyProfileSlotBusy && slot != BusyProfileSlotCustom {
		return fmt.Errorf("profile slot %q is not supported", slot)
	}
	return nil
}

func validateUpdateVersion(version string) error {
	if version == "" || len(version) > maxUpdateVersionBytes {
		return fmt.Errorf("version must be 1-%d bytes", maxUpdateVersionBytes)
	}
	return nil
}

func validFirmwareClock(value string) bool {
	var hour, minute int
	count, err := fmt.Sscanf(value, "%d:%d", &hour, &minute)
	return err == nil && count == 2 && hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59
}

func validateInputKey(key InputKey) error {
	switch key {
	case InputKeyUp, InputKeyDown, InputKeyOK, InputKeyBack, InputKeyStart, InputKeyBusy, InputKeyCustom, InputKeyOff, InputKeyApps, InputKeySettings:
		return nil
	default:
		return fmt.Errorf("key %q is not supported", key)
	}
}

func validDisplayAlign(value DisplayAlign) bool {
	switch value {
	case DisplayAlignTopLeft, DisplayAlignTopMid, DisplayAlignTopRight, DisplayAlignMidLeft, DisplayAlignCenter, DisplayAlignMidRight, DisplayAlignBottomLeft, DisplayAlignBottomMid, DisplayAlignBottomRight:
		return true
	default:
		return false
	}
}

func validFont(value Font) bool {
	switch value {
	case FontTiny, FontSmall, FontNormal, FontCondensed, FontBold, FontLarge, FontExtraLarge, FontGlobal:
		return true
	default:
		return false
	}
}

func validRectangleFill(value RectangleFill) bool {
	switch value {
	case RectangleFillNone, RectangleFillSolid, RectangleFillGradientH, RectangleFillGradientV:
		return true
	default:
		return false
	}
}

func validWiFiConnectSecurity(value WiFiSecurityMethod) bool {
	switch value {
	case WiFiSecurityOpen, WiFiSecurityWPA, WiFiSecurityWPA2, WiFiSecurityWEP, WiFiSecurityWPAWPA2, WiFiSecurityWPA3, WiFiSecurityWPA2WPA3:
		return true
	default:
		return false
	}
}

func validWiFiIPMethod(value WiFiIPMethod) bool {
	switch value {
	case WiFiIPMethodDHCP, WiFiIPMethodStatic:
		return true
	default:
		return false
	}
}

func validAccountClientCertType(value AccountClientCertType) bool {
	switch value {
	case AccountClientCertDefault, AccountClientCertCustom, AccountClientCertNone:
		return true
	default:
		return false
	}
}

func validSmartHomeSwitchStartup(value SmartHomeSwitchStartup) bool {
	switch value {
	case SmartHomeSwitchStartupOff, SmartHomeSwitchStartupOn, SmartHomeSwitchStartupToggle, SmartHomeSwitchStartupLast:
		return true
	default:
		return false
	}
}

func fieldError(field string, err error) error {
	return fmt.Errorf("%s: %w", field, err)
}
