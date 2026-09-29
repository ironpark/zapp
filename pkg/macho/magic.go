package macho

import "encoding/binary"

// HeaderSize is how much of a file IsHeader needs.
const HeaderSize = 8

// IsHeader reports whether header, a file's first HeaderSize bytes, begins a
// Mach-O binary, thin or universal.
func IsHeader(header []byte) bool {
	if len(header) < HeaderSize {
		return false // too short to be a binary
	}
	switch binary.BigEndian.Uint32(header) {
	case magic32, magic64, 0xcefaedfe, 0xcffaedfe:
		return true
	case fatMagic, fatMagic64:
		// Java class files share the universal magic; there the next word is
		// a class file version, 45 or more, where a universal binary counts
		// its few architectures.
		return binary.BigEndian.Uint32(header[4:]) < 45
	}
	return false
}
