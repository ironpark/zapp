package dsstore

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"golang.org/x/text/unicode/norm"
	"howett.net/plist"
	"unicode/utf16"
)

const (
	// EntryTypeBackground represents the background entry type
	EntryTypeBackground = "BKGD"
	// EntryTypeIconLocation represents the icon location entry type
	EntryTypeIconLocation = "Iloc"
	// EntryTypeFinderWindowInfo represents a deprecated Finder window info entry type
	EntryTypeFinderWindowInfo = "fwi0"
	// EntryTypePicture represents the picture entry type
	EntryTypePicture = "pict"
	// EntryTypeWorkspaceSettings represents the workspace settings entry type
	EntryTypeWorkspaceSettings = "bwsp"
	// EntryTypeIconViewPreferences represents the icon view preferences entry type
	EntryTypeIconViewPreferences = "icvp"
	// EntryTypeVersion represents the version entry type
	EntryTypeVersion = "vSrn"
)

type Entry struct {
	Filename  string
	EntryType string
	Buffer    []byte
}

// NewEntry creates a new Entry with the provided parameters.
func NewEntry(filename string, entryType string, dataType string, blob []byte) *Entry {
	filename = normalize(filename)
	filenameLength := len(filename)
	filenameBytes := filenameLength * 2

	buffer := make([]byte, 4+filenameBytes+4+4+len(blob))
	binary.BigEndian.PutUint32(buffer[0:], uint32(filenameLength))
	copy(buffer[4:], utf16be(filename))
	copy(buffer[4+filenameBytes:], entryType)
	copy(buffer[8+filenameBytes:], dataType)
	copy(buffer[12+filenameBytes:], blob)

	return &Entry{
		Filename:  filename,
		EntryType: entryType,
		Buffer:    buffer,
	}
}

// NewBackgroundEntry creates a new background entry.
func NewBackgroundEntry(filename string, opts map[string]interface{}) (*Entry, error) {
	dataType := "blob"
	blob := make([]byte, 12+4)
	binary.BigEndian.PutUint32(blob[0:], uint32(len(blob)-4))
	if opts["color"] != nil {
		copy(blob[4:], "ClrB")
		return nil, fmt.Errorf("not implemented")
	} else if opts["pictureByteLength"] != nil {
		copy(blob[4:], "PctB")
		binary.BigEndian.PutUint32(blob[8:], opts["pictureByteLength"].(uint32))
	} else {
		copy(blob[4:], "DefB")
	}
	return NewEntry(filename, EntryTypeBackground, dataType, blob), nil
}

// NewIconLocationEntry creates a new icon location entry.
func NewIconLocationEntry(filename string, x, y uint32) (*Entry, error) {
	dataType := "blob"
	blob := make([]byte, 16+4)
	binary.BigEndian.PutUint32(blob[0:], uint32(len(blob)-4))
	binary.BigEndian.PutUint32(blob[4:], x)
	binary.BigEndian.PutUint32(blob[8:], y)
	copy(blob[12:], []byte{0xFF, 0xFF, 0xFF, 0x00})
	return NewEntry(filename, EntryTypeIconLocation, dataType, blob), nil
}

// NewWorkspaceSettingsEntry creates a new workspace settings entry.
func NewWorkspaceSettingsEntry(filename string, x, y, width, height int) (*Entry, error) {
	//dataType := "bplist"
	// Replace with bplist creation logic as needed
	buffer := &bytes.Buffer{}
	err := plist.NewBinaryEncoder(buffer).Encode(map[string]any{
		"ContainerShowSidebar": true,
		"ShowPathbar":          false,
		"ShowSidebar":          true,
		"ShowStatusBar":        false,
		"ShowTabView":          false,
		"ShowToolbar":          false,
		"SidebarWidth":         0,
		"WindowBounds":         fmt.Sprintf("{{%d, %d}, {%d, %d}}", x, y, width, height),
	})

	if err != nil {
		return nil, err
	}

	blob := buffer.Bytes()
	newBuff := make([]byte, len(blob)+4)
	binary.BigEndian.PutUint32(newBuff[0:4], uint32(len(blob)))
	copy(newBuff[4:], blob)

	return NewEntry(filename, EntryTypeWorkspaceSettings, "blob", newBuff), nil
}

// NewVersionEntry creates a new version entry.
func NewVersionEntry(filename string, version uint32) (*Entry, error) {
	dataType := "long"
	blob := make([]byte, 4)
	binary.BigEndian.PutUint32(blob, version)
	return NewEntry(filename, EntryTypeVersion, dataType, blob), nil
}

// Helper function to normalize a string (stubbed for example).
func normalize(str string) string {
	return norm.NFD.String(str)
}

func (e Entry) Length() int {
	return len(e.Buffer)
}

type Entries []*Entry

func (e Entries) Len() int {
	return len(e)
}

func (e Entries) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func (e Entries) Less(i, j int) bool {
	if result := hfsPlusFastUnicodeCompare(e[i].Filename, e[j].Filename); result != 0 {
		return result < 0
	}
	if result := hfsPlusFastUnicodeCompare(e[i].EntryType, e[j].EntryType); result != 0 {
		return result < 0
	}
	return false
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
