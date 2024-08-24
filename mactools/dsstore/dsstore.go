// Package dsstore is a package for writing .DS_Store files on macOS.
// Original code from https://github.com/LinusU/node-ds-store (MIT)
package dsstore

import (
	"bytes"
	"encoding/binary"
	"os"
	"sort"
	"unicode/utf16"
	"zapp/mactools/dsstore/entry"

	"github.com/samber/lo"
	"golang.org/x/text/unicode/norm"

	_ "embed"
)

//go:embed clean
var DSStoreClean []byte

type Entries []entry.Entry

func (e Entries) Len() int {
	return len(e)
}

func (e Entries) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func (e Entries) Less(i, j int) bool {
	if result := hfsPlusFastUnicodeCompare(e[i].Filename(), e[j].Filename()); result != 0 {
		return result < 0
	}
	if result := hfsPlusFastUnicodeCompare(e[i].EntryType(), e[j].EntryType()); result != 0 {
		return result < 0
	}
	return false
}

type DSStore struct {
	Entries []entry.Entry
}

func NewDSStore() *DSStore {
	return &DSStore{
		Entries: make([]entry.Entry, 0),
	}
}
func (ds *DSStore) getIconViewPreferences() *entry.IconViewPreferencesEntry {
	e, ok := lo.Find(ds.Entries, func(e entry.Entry) bool {
		return e.EntryType() == entry.TypeIconViewPreferences
	})
	if ok {
		return e.(*entry.IconViewPreferencesEntry)
	}
	newEntry := entry.NewIconViewPreferencesEntry(100)
	ds.AddEntry(newEntry)
	return newEntry
}

func (ds *DSStore) SetLabelPlaceToBottom(bottom bool) {
	ds.getIconViewPreferences().LabelOnBottom = bottom
}

func (ds *DSStore) SetLabelSize(size float64) {
	ds.getIconViewPreferences().TextSize = size
}

func (ds *DSStore) SetIconSize(size float64) {
	ds.getIconViewPreferences().IconSize = size
}

func (ds *DSStore) SetBgColor(r, g, b float64) {
	ds.getIconViewPreferences().SetBgColor(r, g, b)
}

func (ds *DSStore) SetWindow(width, height, x, y int) {
	e, ok := lo.Find(ds.Entries, func(e entry.Entry) bool {
		return e.EntryType() == entry.TypeWorkspaceSettings
	})
	if ok {
		e.(*entry.WorkspaceSettingsEntry).Width = width
		e.(*entry.WorkspaceSettingsEntry).Height = height
		e.(*entry.WorkspaceSettingsEntry).X = x
		e.(*entry.WorkspaceSettingsEntry).Y = y
	} else {
		newEntry := entry.NewWorkspaceSettingsEntry(x, y, width, height)
		ds.AddEntry(newEntry)
	}
}

func (ds *DSStore) SetIconPos(name string, x, y uint32) {
	e, ok := lo.Find(ds.Entries, func(e entry.Entry) bool {
		return e.Filename() == name && e.EntryType() == entry.TypeIconLocation
	})
	if ok {
		e.(*entry.IconLocationEntry).X = x
		e.(*entry.IconLocationEntry).Y = y
	} else {
		newEntry := entry.NewIconLocationEntry(name, x, y)
		ds.AddEntry(newEntry)
	}
}

func (ds *DSStore) AddEntry(entry entry.Entry) {
	ds.Entries = append(ds.Entries, entry)
}

func (ds *DSStore) Write(filePath string) error {
	sort.Sort(Entries(ds.Entries))

	buf := bytes.Clone(DSStoreClean)
	modified := make([]byte, 3840)
	currentPos := 0
	P := uint32(0)
	count := uint32(len(ds.Entries))

	binary.BigEndian.PutUint32(modified[currentPos:], P)
	binary.BigEndian.PutUint32(modified[currentPos+4:], count)
	currentPos += 8

	for _, entry := range ds.Entries {
		blob := entryBuild(entry)
		copy(modified[currentPos:], blob)
		currentPos += len(blob)
	}

	binary.BigEndian.PutUint32(buf[76:], count)
	copy(buf[4100:], modified)
	return os.WriteFile(filePath, buf, 0644)
}

func entryBuild(entry entry.Entry) []byte {
	filename := norm.NFD.String(entry.Filename())
	filenameLength := len(filename)
	filenameBytes := filenameLength * 2
	blob := entry.Bytes()
	buffer := make([]byte, 4+filenameBytes+4+4+len(blob))
	binary.BigEndian.PutUint32(buffer[0:], uint32(filenameLength))
	copy(buffer[4:], utf16be(filename))
	copy(buffer[4+filenameBytes:], entry.EntryType())
	copy(buffer[8+filenameBytes:], entry.DataType())
	copy(buffer[12+filenameBytes:], blob)
	return buffer
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
