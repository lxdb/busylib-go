package busylib

import (
	"strings"
	"unicode"
)

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

// NewAssetAudio creates a playback request for an uploaded asset.
func NewAssetAudio(applicationName, path string) PlayAudio {
	return PlayAudio{
		ApplicationName: applicationName,
		Path:            path,
	}
}

// NewStockAudio creates a playback request for a firmware stock asset.
func NewStockAudio(applicationName, stockPath string) PlayAudio {
	return PlayAudio{
		ApplicationName: applicationName,
		StockPath:       stockPath,
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
