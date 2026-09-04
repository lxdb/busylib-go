package busylib

import (
	"encoding/json"
	"strings"
	"unicode"

	"github.com/lxdb/busylib-go/display"
)

// DisplayElement is a supported element in a display update.
type DisplayElement interface {
	displayElement()
}

// BaseDisplayElement contains placement and lifetime fields shared by display
// elements. Positive Timeout and DisplayUntil values are mutually exclusive.
// Nil coordinates omit explicit placement on the corresponding axis.
type BaseDisplayElement struct {
	ID           string `json:"id"`
	Timeout      *int   `json:"timeout,omitempty"`
	DisplayUntil string `json:"display_until,omitempty"`
	X            *int   `json:"x,omitempty"`
	Y            *int   `json:"y,omitempty"`
	// ZIndex selects a layer from 0 through 2,147,483,647. Nil lets the firmware
	// assign a layer from the element's request order.
	ZIndex  *int          `json:"z_index,omitempty"`
	Display DisplayTarget `json:"display,omitempty"`
	Align   DisplayAlign  `json:"align,omitempty"`
}

// DisplayTarget selects a physical device display.
type DisplayTarget = display.Target

const (
	// DisplayFront targets the front display.
	DisplayFront = display.Front
	// DisplayBack targets the back display.
	DisplayBack = display.Back
)

// DisplayAlign selects the anchor point for an element.
type DisplayAlign string

const (
	// DisplayAlignTopLeft anchors the element at the top left.
	DisplayAlignTopLeft DisplayAlign = "top_left"
	// DisplayAlignTopMid anchors the element at the top center.
	DisplayAlignTopMid DisplayAlign = "top_mid"
	// DisplayAlignTopRight anchors the element at the top right.
	DisplayAlignTopRight DisplayAlign = "top_right"
	// DisplayAlignMidLeft anchors the element at the middle left.
	DisplayAlignMidLeft DisplayAlign = "mid_left"
	// DisplayAlignCenter anchors the element at the center.
	DisplayAlignCenter DisplayAlign = "center"
	// DisplayAlignMidRight anchors the element at the middle right.
	DisplayAlignMidRight DisplayAlign = "mid_right"
	// DisplayAlignBottomLeft anchors the element at the bottom left.
	DisplayAlignBottomLeft DisplayAlign = "bottom_left"
	// DisplayAlignBottomMid anchors the element at the bottom center.
	DisplayAlignBottomMid DisplayAlign = "bottom_mid"
	// DisplayAlignBottomRight anchors the element at the bottom right.
	DisplayAlignBottomRight DisplayAlign = "bottom_right"
)

type displayElementType string

const (
	displayElementText      displayElementType = "text"
	displayElementImage     displayElementType = "image"
	displayElementAnimation displayElementType = "animation"
	displayElementCountdown displayElementType = "countdown"
	displayElementRectangle displayElementType = "rectangle"
	displayElementXPMBitmap displayElementType = "xpmbitmap"
)

// TextElement displays text with the selected font and scrolling behavior.
type TextElement struct {
	BaseDisplayElement
	Text              string `json:"text"`
	Font              Font   `json:"font"`
	Color             string `json:"color,omitempty"`
	Width             int    `json:"width,omitempty"`
	ScrollRate        int    `json:"scroll_rate,omitempty"`
	ScrollStartDelay  int    `json:"scroll_start_delay,omitempty"`
	ScrollRepeatDelay int    `json:"scroll_repeat_delay,omitempty"`
}

// Font selects a firmware-provided display font.
type Font string

const (
	// FontTiny selects the tiny font.
	FontTiny Font = "tiny"
	// FontSmall selects the small font.
	FontSmall Font = "small"
	// FontNormal selects the normal font.
	FontNormal Font = "normal"
	// FontCondensed selects the condensed font.
	FontCondensed Font = "condensed"
	// FontBold selects the bold font.
	FontBold Font = "bold"
	// FontLarge selects the large font.
	FontLarge Font = "large"
	// FontExtraLarge selects the extra-large font.
	FontExtraLarge Font = "extra_large"
	// FontGlobal selects the device global font.
	FontGlobal Font = "global"
)

// ImageElement displays a stored or stock image.
type ImageElement struct {
	BaseDisplayElement
	Path      string `json:"path,omitempty"`
	StockPath string `json:"stock_path,omitempty"`
	Opacity   *int   `json:"opacity,omitempty"`
}

// AnimationElement displays a stored or stock animation.
type AnimationElement struct {
	BaseDisplayElement
	Path             string `json:"path,omitempty"`
	StockPath        string `json:"stock_path,omitempty"`
	Loop             bool   `json:"loop,omitempty"`
	AwaitPreviousEnd bool   `json:"await_previous_end,omitempty"`
	Section          string `json:"section,omitempty"`
	Opacity          *int   `json:"opacity,omitempty"`
}

// XPMBitmapElement displays inline XPM bitmap data.
type XPMBitmapElement struct {
	BaseDisplayElement
	// Data contains the inline XPM2 image.
	Data string `json:"data"`
	// Opacity optionally selects a percentage from 0 through 100.
	Opacity *int `json:"opacity,omitempty"`
}

// CountdownElement displays elapsed or remaining time for a timestamp.
type CountdownElement struct {
	BaseDisplayElement
	Timestamp string             `json:"timestamp"`
	Color     string             `json:"color,omitempty"`
	Direction CountdownDirection `json:"direction"`
	ShowHours CountdownShowHours `json:"show_hours"`
}

// CountdownDirection selects elapsed or remaining time.
type CountdownDirection string

const (
	// CountdownTimeLeft shows time remaining until the timestamp.
	CountdownTimeLeft CountdownDirection = "time_left"
	// CountdownTimeSince shows time elapsed since the timestamp.
	CountdownTimeSince CountdownDirection = "time_since"
)

// CountdownShowHours controls when a countdown includes hours.
type CountdownShowHours string

const (
	// CountdownShowHoursWhenNonZero hides a zero hours field.
	CountdownShowHoursWhenNonZero CountdownShowHours = "when_non_zero"
	// CountdownShowHoursAlways always shows the hours field.
	CountdownShowHoursAlways CountdownShowHours = "always"
)

// RectangleElement displays a filled or outlined rectangle.
type RectangleElement struct {
	BaseDisplayElement
	Width       int           `json:"width"`
	Height      int           `json:"height"`
	Radius      int           `json:"radius,omitempty"`
	Fill        RectangleFill `json:"fill,omitempty"`
	FillColors  []string      `json:"fill_colors,omitempty"`
	BorderWidth *int          `json:"border_width,omitempty"`
	BorderColor string        `json:"border_color,omitempty"`
}

// RectangleFill selects the rectangle fill style.
type RectangleFill string

const (
	// RectangleFillNone leaves the rectangle transparent.
	RectangleFillNone RectangleFill = "none"
	// RectangleFillSolid uses one fill color.
	RectangleFillSolid RectangleFill = "solid"
	// RectangleFillGradientH uses a horizontal gradient.
	RectangleFillGradientH RectangleFill = "gradient_h"
	// RectangleFillGradientV uses a vertical gradient.
	RectangleFillGradientV RectangleFill = "gradient_v"
)

func (TextElement) displayElement()      {}
func (ImageElement) displayElement()     {}
func (AnimationElement) displayElement() {}
func (CountdownElement) displayElement() {}
func (RectangleElement) displayElement() {}
func (XPMBitmapElement) displayElement() {}

// MarshalJSON adds the text element wire discriminator.
func (e TextElement) MarshalJSON() ([]byte, error) {
	type alias TextElement
	return json.Marshal(struct {
		Type displayElementType `json:"type"`
		alias
	}{Type: displayElementText, alias: alias(e)})
}

// MarshalJSON adds the image element wire discriminator.
func (e ImageElement) MarshalJSON() ([]byte, error) {
	type alias ImageElement
	return json.Marshal(struct {
		Type displayElementType `json:"type"`
		alias
	}{Type: displayElementImage, alias: alias(e)})
}

// MarshalJSON adds the animation element wire discriminator.
func (e AnimationElement) MarshalJSON() ([]byte, error) {
	type alias AnimationElement
	return json.Marshal(struct {
		Type displayElementType `json:"type"`
		alias
	}{Type: displayElementAnimation, alias: alias(e)})
}

// MarshalJSON adds the countdown element wire discriminator.
func (e CountdownElement) MarshalJSON() ([]byte, error) {
	type alias CountdownElement
	return json.Marshal(struct {
		Type displayElementType `json:"type"`
		alias
	}{Type: displayElementCountdown, alias: alias(e)})
}

// MarshalJSON adds the rectangle element wire discriminator.
func (e RectangleElement) MarshalJSON() ([]byte, error) {
	type alias RectangleElement
	return json.Marshal(struct {
		Type displayElementType `json:"type"`
		alias
	}{Type: displayElementRectangle, alias: alias(e)})
}

// MarshalJSON adds the XPM bitmap element wire discriminator.
func (e XPMBitmapElement) MarshalJSON() ([]byte, error) {
	type alias XPMBitmapElement
	return json.Marshal(struct {
		Type displayElementType `json:"type"`
		alias
	}{Type: displayElementXPMBitmap, alias: alias(e)})
}

// NewTextElement creates a text element for the front display.
func NewTextElement(id, text string, font Font) TextElement {
	return TextElement{
		BaseDisplayElement: defaultDisplayElement(id),
		Text:               text,
		Font:               font,
	}
}

// NewAssetImageElement creates a front-display image from an uploaded asset.
func NewAssetImageElement(id, path string) ImageElement {
	return ImageElement{
		BaseDisplayElement: defaultDisplayElement(id),
		Path:               path,
	}
}

// NewStockImageElement creates a front-display image from a firmware stock asset.
func NewStockImageElement(id, stockPath string) ImageElement {
	return ImageElement{
		BaseDisplayElement: defaultDisplayElement(id),
		StockPath:          stockPath,
	}
}

// NewAssetAnimationElement creates a front-display animation from an uploaded asset.
func NewAssetAnimationElement(id, path string) AnimationElement {
	return AnimationElement{
		BaseDisplayElement: defaultDisplayElement(id),
		Path:               path,
	}
}

// NewStockAnimationElement creates a front-display animation from a firmware stock asset.
func NewStockAnimationElement(id, stockPath string) AnimationElement {
	return AnimationElement{
		BaseDisplayElement: defaultDisplayElement(id),
		StockPath:          stockPath,
	}
}

// NewXPMBitmapElement creates an inline XPM bitmap for the front display.
func NewXPMBitmapElement(id, data string) XPMBitmapElement {
	return XPMBitmapElement{
		BaseDisplayElement: defaultDisplayElement(id),
		Data:               data,
	}
}

// NewCountdownElement creates a front-display countdown with the supplied time settings.
func NewCountdownElement(id, timestamp string, direction CountdownDirection, showHours CountdownShowHours) CountdownElement {
	return CountdownElement{
		BaseDisplayElement: defaultDisplayElement(id),
		Timestamp:          timestamp,
		Direction:          direction,
		ShowHours:          showHours,
	}
}

// NewRectangleElement creates a rectangle for the front display.
func NewRectangleElement(id string, width, height int) RectangleElement {
	return RectangleElement{
		BaseDisplayElement: defaultDisplayElement(id),
		Width:              width,
		Height:             height,
	}
}

// SanitizeDisplayText removes control characters and unsupported display symbols.
// It also collapses consecutive white space to one ASCII space.
func SanitizeDisplayText(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsSpace(r):
			builder.WriteByte(' ')
		case unicode.IsControl(r) || isDisplaySymbol(r):
			continue
		default:
			builder.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func defaultDisplayElement(id string) BaseDisplayElement {
	return BaseDisplayElement{
		ID:      id,
		Display: DisplayFront,
	}
}

func isDisplaySymbol(r rune) bool {
	return (r >= 0x1f300 && r <= 0x1faff) ||
		(r >= 0x2600 && r <= 0x27bf) ||
		r == 0x200d ||
		r == 0xfe0f
}
