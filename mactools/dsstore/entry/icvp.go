package entry

import (
	"bytes"
	"encoding/binary"
	"github.com/ironpark/zapp/mactools/alias"
	"unicode/utf16"

	"howett.net/plist"
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

	buffer := &bytes.Buffer{}
	err := plist.NewBinaryEncoder(buffer).Encode(base)
	if err != nil {
		return nil
	}
	return plistWrap(buffer.Bytes())
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

func (i *IconViewPreferencesEntry) SetBgImage(imagePath string) (err error) {
	i.BackgroundType = 2
	i.BackgroundImageAlias, err = alias.Create(imagePath)
	return err
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

// utf16be converts a string to a big-endian UTF-16 byte slice.
func utf16be(str string) []byte {
	utf16Encoded := utf16.Encode([]rune(str))
	buffer := new(bytes.Buffer)
	for _, r := range utf16Encoded {
		binary.Write(buffer, binary.BigEndian, r)
	}
	return buffer.Bytes()
}
