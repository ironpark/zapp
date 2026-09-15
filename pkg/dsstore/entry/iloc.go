package entry

import "encoding/binary"

type IconLocationEntry struct {
	X        uint32
	Y        uint32
	filename string
}

func (i IconLocationEntry) Bytes() []byte {
	blob := make([]byte, 16+4)
	binary.BigEndian.PutUint32(blob[0:], uint32(len(blob)-4))
	binary.BigEndian.PutUint32(blob[4:], i.X)
	binary.BigEndian.PutUint32(blob[8:], i.Y)
	copy(blob[12:], []byte{0xFF, 0xFF, 0xFF, 0x00})
	return blob
}

func (i IconLocationEntry) Filename() string {
	return i.filename
}

func (i IconLocationEntry) EntryType() string {
	return TypeIconLocation
}

func (i IconLocationEntry) DataType() string {
	return "blob"
}

// NewIconLocationEntry creates a new icon location entry.
func NewIconLocationEntry(filename string, x, y uint32) *IconLocationEntry {
	return &IconLocationEntry{
		X:        x,
		Y:        y,
		filename: filename,
	}
}
