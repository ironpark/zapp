package entry

import (
	"bytes"

	"howett.net/plist"
)

type IconViewPreferencesEntry struct {
	BackgroundType       int
	BackgroundColorRed   float64
	BackgroundColorGreen float64
	BackgroundColorBlue  float64
	ShowIconPreview      bool
	ShowItemInfo         bool
	TextSize             float64
	IconSize             float64
	ViewOptionsVersion   int
	GridSpacing          float64
	GridOffsetX          float64
	GridOffsetY          float64
	LabelOnBottom        bool
	ArrangeBy            string
}

func (i *IconViewPreferencesEntry) Bytes() []byte {
	base := map[string]any{
		"backgroundType":       i.BackgroundType,
		"backgroundColorRed":   i.BackgroundColorRed,
		"backgroundColorGreen": i.BackgroundColorGreen,
		"backgroundColorBlue":  i.BackgroundColorBlue,
		"showIconPreview":      i.ShowIconPreview,
		"showItemInfo":         i.ShowItemInfo,
		"textSize":             i.TextSize,
		"iconSize":             i.IconSize,
		"viewOptionsVersion":   i.ViewOptionsVersion,
		"gridSpacing":          i.GridSpacing,
		"gridOffsetX":          i.GridOffsetX,
		"gridOffsetY":          i.GridOffsetY,
		"labelOnBottom":        i.LabelOnBottom,
		"arrangeBy":            i.ArrangeBy,
	}
	buffer := &bytes.Buffer{}
	err := plist.NewBinaryEncoder(buffer).Encode(base)
	if err != nil {
		return nil
	}
	return plistWrap(buffer.Bytes())
}

func (i *IconViewPreferencesEntry) SetBgToDefault() {
	i.BackgroundType = 0
}

func (i *IconViewPreferencesEntry) SetBgColor(r, g, b float64) {
	i.BackgroundType = 1
	i.BackgroundColorRed = r
	i.BackgroundColorGreen = g
	i.BackgroundColorBlue = b
}

func (i *IconViewPreferencesEntry) Filename() string {
	return "."
}

func (i *IconViewPreferencesEntry) EntryType() string {
	return TypeIconViewPreferences
}

func (i *IconViewPreferencesEntry) DataType() string {
	return "blob"
}

// NewIconViewPreferencesEntry creates a new icon view preferences entry.
func NewIconViewPreferencesEntry(iconSize float64) *IconViewPreferencesEntry {
	return &IconViewPreferencesEntry{
		BackgroundType:       1,
		BackgroundColorRed:   1,
		BackgroundColorGreen: 1,
		BackgroundColorBlue:  1,
		ShowIconPreview:      true,
		ShowItemInfo:         true,
		TextSize:             12,
		IconSize:             iconSize,
		ViewOptionsVersion:   1,
		GridSpacing:          100,
		GridOffsetX:          0,
		GridOffsetY:          0,
		LabelOnBottom:        false,
		ArrangeBy:            "none",
	}
}
