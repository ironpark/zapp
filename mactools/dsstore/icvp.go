package dsstore

import (
	"bytes"
	"encoding/binary"
	"howett.net/plist"
)

type iconViewPreferences struct {
	backgroundType       int
	backgroundColorRed   float64
	backgroundColorGreen float64
	backgroundColorBlue  float64
	showIconPreview      bool
	showItemInfo         bool
	textSize             float64
	iconSize             float64
	viewOptionsVersion   int
	gridSpacing          float64
	gridOffsetX          float64
	gridOffsetY          float64
	labelOnBottom        bool
	arrangeBy            string
}

// NewIconViewPreferencesEntry creates a new icon view preferences entry.
func NewIconViewPreferencesEntry(filename string, iconSize float64) (*Entry, error) {
	cf := iconViewPreferences{
		backgroundType:       1,
		backgroundColorRed:   1,
		backgroundColorGreen: 1,
		backgroundColorBlue:  1,
		showIconPreview:      true,
		showItemInfo:         false,
		textSize:             12,
		iconSize:             iconSize,
		viewOptionsVersion:   1,
		gridSpacing:          100,
		gridOffsetX:          0,
		gridOffsetY:          0,
		labelOnBottom:        true,
		arrangeBy:            "none",
	}
	//dataType := "bplist"
	buffer := &bytes.Buffer{}
	err := plist.NewBinaryEncoder(buffer).Encode(map[string]any{
		"backgroundType":       cf.backgroundType,
		"backgroundColorRed":   cf.backgroundColorRed,
		"backgroundColorGreen": cf.backgroundColorGreen,
		"backgroundColorBlue":  cf.backgroundColorBlue,
		"showIconPreview":      cf.showIconPreview,
		"showItemInfo":         cf.showItemInfo,
		"textSize":             cf.textSize,
		"iconSize":             cf.iconSize,
		"viewOptionsVersion":   cf.viewOptionsVersion,
		"gridSpacing":          cf.gridSpacing,
		"gridOffsetX":          cf.gridOffsetX,
		"gridOffsetY":          cf.gridOffsetY,
		"labelOnBottom":        cf.labelOnBottom,
		"arrangeBy":            cf.arrangeBy,
	})
	if err != nil {
		return nil, err
	}

	blob := buffer.Bytes()
	newBuff := make([]byte, len(blob)+4)
	binary.BigEndian.PutUint32(newBuff[0:4], uint32(len(blob)))
	copy(newBuff[4:], blob)

	return NewEntry(filename, EntryTypeIconViewPreferences, "blob", newBuff), nil
}
