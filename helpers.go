package busylib

import (
	"strings"
	"unicode"
)

const DefaultDisplayPriority = 50

func NewDisplayElements(applicationName string, elements ...DisplayElement) DisplayElements {
	return DisplayElements{
		ApplicationName: applicationName,
		Priority:        DefaultDisplayPriority,
		Elements:        elements,
	}
}

func NewTextElement(id, text string, font Font) TextElement {
	return TextElement{
		BaseDisplayElement: defaultDisplayElement(id),
		Text:               text,
		Font:               font,
	}
}

func NewAssetImageElement(id, path string) ImageElement {
	return ImageElement{
		BaseDisplayElement: defaultDisplayElement(id),
		Path:               path,
	}
}

func NewStockImageElement(id, stockPath string) ImageElement {
	return ImageElement{
		BaseDisplayElement: defaultDisplayElement(id),
		StockPath:          stockPath,
	}
}

func NewAssetAnimationElement(id, path string) AnimationElement {
	return AnimationElement{
		BaseDisplayElement: defaultDisplayElement(id),
		Path:               path,
	}
}

func NewStockAnimationElement(id, stockPath string) AnimationElement {
	return AnimationElement{
		BaseDisplayElement: defaultDisplayElement(id),
		StockPath:          stockPath,
	}
}

func NewCountdownElement(id, timestamp string, direction CountdownDirection, showHours CountdownShowHours) CountdownElement {
	return CountdownElement{
		BaseDisplayElement: defaultDisplayElement(id),
		Timestamp:          timestamp,
		Direction:          direction,
		ShowHours:          showHours,
	}
}

func NewRectangleElement(id string, width, height int) RectangleElement {
	return RectangleElement{
		BaseDisplayElement: defaultDisplayElement(id),
		Width:              width,
		Height:             height,
	}
}

func NewAssetAudio(applicationName, path string) PlayAudio {
	return PlayAudio{
		ApplicationName: applicationName,
		Path:            path,
	}
}

func NewStockAudio(applicationName, stockPath string) PlayAudio {
	return PlayAudio{
		ApplicationName: applicationName,
		StockPath:       stockPath,
	}
}

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
