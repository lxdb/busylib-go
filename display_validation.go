package busylib

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	colorPattern            = regexp.MustCompile(`^#[a-fA-F0-9]{8}$`)
	brightnessPattern       = regexp.MustCompile(`^(auto|[0-9]{1,2}|100)$`)
	displayElementIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

const maxDisplayElements = 100

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
	if err := validateApplicationName(request.ApplicationName); err != nil {
		return err
	}
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
	case XPMBitmapElement:
		return value.BaseDisplayElement, func(index int) error { return validateXPMBitmapElement(index, value) }, nil
	case *XPMBitmapElement:
		if value == nil {
			return BaseDisplayElement{}, nil, fmt.Errorf("elements[%d] must not be nil", index)
		}
		return value.BaseDisplayElement, func(index int) error { return validateXPMBitmapElement(index, *value) }, nil
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
	if base.ZIndex != nil && (*base.ZIndex < 0 || *base.ZIndex > math.MaxInt32) {
		return fmt.Errorf("%s.z_index must be between 0 and %d", prefix, math.MaxInt32)
	}
	if base.Display != "" && base.Display != DisplayFront && base.Display != DisplayBack {
		return fmt.Errorf("%s.display %q is not supported", prefix, base.Display)
	}
	if base.Align != "" && !validDisplayAlign(base.Align) {
		return fmt.Errorf("%s.align %q is not supported", prefix, base.Align)
	}
	return nil
}

func validateXPMBitmapElement(index int, element XPMBitmapElement) error {
	prefix := fmt.Sprintf("elements[%d]", index)
	if element.Data == "" {
		return fmt.Errorf("%s.data must not be empty", prefix)
	}
	return validateOptionalPercent(prefix+".opacity", element.Opacity)
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

func validateOptionalPercent(field string, value *int) error {
	if value == nil {
		return nil
	}
	if *value < 0 || *value > 100 {
		return fmt.Errorf("%s must be between 0 and 100", field)
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

func validateScreenDisplay(display DisplayTarget) error {
	if display != DisplayFront && display != DisplayBack {
		return errors.New("display must be front or back")
	}
	return nil
}

func validateClearDisplayElementsRequest(request ClearDisplayElementsRequest) error {
	if len(request.ElementIDs) == 0 {
		return errors.New("element_ids must contain at least one element ID")
	}
	if request.ApplicationName != "" {
		if err := validateApplicationName(request.ApplicationName); err != nil {
			return err
		}
	}
	for index, id := range request.ElementIDs {
		if !displayElementIDPattern.MatchString(id) {
			return fmt.Errorf("element_ids[%d] must contain only ASCII letters, digits, periods, underscores, or hyphens", index)
		}
	}
	return nil
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
