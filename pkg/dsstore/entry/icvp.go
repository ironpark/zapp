package entry

import (
	"github.com/ironpark/zapp/pkg/plist"
)

type IconViewPreferencesEntry struct {
	BackgroundType       int
	BackgroundColorRed   float64
	BackgroundColorGreen float64
	BackgroundColorBlue  float64
	BackgroundImageAlias []byte // 배경 이미지 경로 추가
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

	// 배경 이미지가 설정된 경우 추가
	if i.BackgroundType == 2 && i.BackgroundImageAlias != nil {
		base["backgroundImageAlias"] = i.BackgroundImageAlias
	}

	data, err := plist.MarshalBinary(base)
	if err != nil {
		return nil
	}
	return plistWrap(data)
}

func (i *IconViewPreferencesEntry) SetBgToDefault() {
	i.BackgroundType = 0
	i.BackgroundImageAlias = nil
}

func (i *IconViewPreferencesEntry) SetBgColor(r, g, b float64) {
	i.BackgroundType = 1
	i.BackgroundColorRed = r
	i.BackgroundColorGreen = g
	i.BackgroundColorBlue = b
	i.BackgroundImageAlias = nil
}

// SetBgImage points the view at a background image. volumeName names the volume
// imagePath lives on, which the alias record records alongside the path.
// SetBgImage points the window at a background image described by an alias
// record the caller has already built.
func (i *IconViewPreferencesEntry) SetBgImage(record []byte) {
	i.BackgroundType = 2
	i.BackgroundImageAlias = record
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
		BackgroundType:       1, // 기본값을 사용자 지정 색상으로 설정
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
		BackgroundImageAlias: nil,
	}
}
