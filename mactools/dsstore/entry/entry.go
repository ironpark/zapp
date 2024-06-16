package entry

import (
	"encoding/binary"
)

const (
	// TypeBackground represents the background entry type
	TypeBackground = "BKGD"
	// TypeIconLocation represents the icon location entry type
	TypeIconLocation = "Iloc"
	// TypeFinderWindowInfo represents a deprecated Finder window info entry type
	TypeFinderWindowInfo = "fwi0"
	// TypePicture represents the picture entry type
	TypePicture = "pict"
	// TypeWorkspaceSettings represents the workspace settings entry type
	TypeWorkspaceSettings = "bwsp"
	// TypeIconViewPreferences represents the icon view preferences entry type
	TypeIconViewPreferences = "icvp"
	// TypeVersion represents the version entry type
	TypeVersion = "vSrn"
)

type Entry interface {
	Bytes() []byte
	Filename() string
	EntryType() string
	DataType() string
}

type EntryItem struct {
	filename  string
	entryType string
	Buffer    []byte
}

func (e EntryItem) Filename() string {
	return e.filename
}

func (e EntryItem) EntryType() string {
	return e.entryType
}

func (e EntryItem) Bytes() []byte {
	return e.Buffer
}

func plistWrap(buffer []byte) []byte {
	newBuff := make([]byte, len(buffer)+4)
	binary.BigEndian.PutUint32(newBuff[0:4], uint32(len(buffer)))
	copy(newBuff[4:], buffer)
	return newBuff
}
