package dmg

// Custom Finder icons.
//
// The Finder reads an icon from a classic Resource Manager resource fork, which
// on modern filesystems is stored in the com.apple.ResourceFork extended
// attribute, and only looks for it when the kHasCustomIcon flag is set in the
// com.apple.FinderInfo attribute. Both are written here directly, replacing a
// round trip through sips, DeRez, Rez and SetFile.

import (
	"encoding/binary"
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

const (
	resourceForkAttr = "com.apple.ResourceFork"
	finderInfoAttr   = "com.apple.FinderInfo"

	// finderInfoSize is the fixed size of com.apple.FinderInfo: a 16-byte
	// FileInfo or FolderInfo followed by 16 bytes of extended info.
	finderInfoSize = 32
	// finderFlagsOffset is where the Finder flags sit. FileInfo and FolderInfo
	// disagree on the preceding fields but both put the flags here.
	finderFlagsOffset = 8
	// creatorOffset is FileInfo.fileCreator. Only meaningful for files.
	creatorOffset = 4

	// hasCustomIcon is kHasCustomIcon, the flag SetFile -a C sets.
	hasCustomIcon = 0x0400

	// iconResourceID is kCustomIconResource, the well-known ID the Finder
	// looks up in the 'icns' resource type.
	iconResourceID   = -16455
	iconResourceType = "icns"
)

// applyCustomIcon stores icns as path's custom icon and marks path as having
// one. It is the equivalent of Rez-ing an 'icns' resource onto the file and
// running SetFile -a C.
func applyCustomIcon(path string, icns []byte) error {
	if len(icns) == 0 {
		return errors.New("icon data is empty")
	}
	fork, err := buildResourceFork(iconResourceType, iconResourceID, icns)
	if err != nil {
		return err
	}
	if err := unix.Setxattr(path, resourceForkAttr, fork, 0); err != nil {
		return fmt.Errorf("failed to write resource fork of %s: %w", path, err)
	}
	return markCustomIcon(path)
}

// markCustomIcon sets the kHasCustomIcon Finder flag on path. A volume root
// keeps its icon in a .VolumeIcon.icns file rather than a resource fork, so it
// needs only this flag.
func markCustomIcon(path string) error {
	info, err := finderInfo(path)
	if err != nil {
		return err
	}
	flags := binary.BigEndian.Uint16(info[finderFlagsOffset:])
	if flags&hasCustomIcon != 0 {
		return nil
	}
	binary.BigEndian.PutUint16(info[finderFlagsOffset:], flags|hasCustomIcon)
	if err := unix.Setxattr(path, finderInfoAttr, info, 0); err != nil {
		return fmt.Errorf("failed to set custom icon flag on %s: %w", path, err)
	}
	return nil
}

// setCreatorCode sets the four character creator code in path's Finder info,
// the equivalent of SetFile -c.
func setCreatorCode(path, code string) error {
	if len(code) != 4 {
		return fmt.Errorf("creator code must be 4 characters, got %q", code)
	}
	info, err := finderInfo(path)
	if err != nil {
		return err
	}
	copy(info[creatorOffset:creatorOffset+4], code)
	if err := unix.Setxattr(path, finderInfoAttr, info, 0); err != nil {
		return fmt.Errorf("failed to set creator code on %s: %w", path, err)
	}
	return nil
}

// finderInfo reads path's Finder info, returning 32 zero bytes when the
// attribute is not present yet. Existing fields are preserved so that setting
// one does not clear the others.
func finderInfo(path string) ([]byte, error) {
	info := make([]byte, finderInfoSize)
	n, err := unix.Getxattr(path, finderInfoAttr, info)
	if err != nil {
		if errors.Is(err, unix.ENOATTR) {
			return info, nil
		}
		return nil, fmt.Errorf("failed to read finder info of %s: %w", path, err)
	}
	// A short attribute is padded rather than rejected; the Finder treats the
	// missing tail as zero.
	for i := n; i < finderInfoSize; i++ {
		info[i] = 0
	}
	return info, nil
}

// Resource fork layout constants. See "Resource Manager" in Inside Macintosh:
// More Macintosh Toolbox.
const (
	// resourceDataOffset is where the data section starts. The header proper is
	// 16 bytes; the rest of the first 256 are reserved.
	resourceDataOffset = 256
	resourceHeaderSize = 256

	// Offsets within the resource map.
	mapTypeListOffset = 28 // the type list follows the map header
	typeEntrySize     = 8
	refEntrySize      = 12
)

// buildResourceFork encodes data as the single resource of the given type and
// ID in a classic resource fork.
func buildResourceFork(resType string, id int16, data []byte) ([]byte, error) {
	if len(resType) != 4 {
		return nil, fmt.Errorf("resource type must be 4 characters, got %q", resType)
	}
	if uint64(len(data)) > 0xFFFFFF {
		// The reference list stores the data offset in 24 bits, so a fork this
		// large could not address a second resource anyway.
		return nil, fmt.Errorf("resource of %d bytes is too large", len(data))
	}

	// Data section: each resource is a big-endian length followed by its bytes.
	dataLen := 4 + len(data)

	// Map section: header, one type entry, one reference entry, empty name list.
	refListOffset := 2 + typeEntrySize // from the start of the type list
	nameListOffset := mapTypeListOffset + refListOffset + refEntrySize
	mapLen := nameListOffset

	fork := make([]byte, resourceHeaderSize+dataLen+mapLen)

	// Header.
	binary.BigEndian.PutUint32(fork[0:], resourceDataOffset)
	binary.BigEndian.PutUint32(fork[4:], uint32(resourceDataOffset+dataLen))
	binary.BigEndian.PutUint32(fork[8:], uint32(dataLen))
	binary.BigEndian.PutUint32(fork[12:], uint32(mapLen))

	// Data section.
	d := fork[resourceDataOffset:]
	binary.BigEndian.PutUint32(d[0:], uint32(len(data)))
	copy(d[4:], data)

	// Map. The first 22 bytes are reserved for in-memory use and stay zero.
	m := fork[resourceDataOffset+dataLen:]
	binary.BigEndian.PutUint16(m[24:], uint16(mapTypeListOffset))
	binary.BigEndian.PutUint16(m[26:], uint16(nameListOffset))

	// Type list: one type, whose count fields are stored minus one.
	t := m[mapTypeListOffset:]
	binary.BigEndian.PutUint16(t[0:], 0) // 1 type
	copy(t[2:], resType)
	binary.BigEndian.PutUint16(t[6:], 0) // 1 resource of this type
	binary.BigEndian.PutUint16(t[8:], uint16(refListOffset))

	// Reference list: one resource, unnamed, at the start of the data section.
	r := m[mapTypeListOffset+refListOffset:]
	binary.BigEndian.PutUint16(r[0:], uint16(id))
	binary.BigEndian.PutUint16(r[2:], 0xFFFF) // no name
	r[4] = 0                                  // no attributes
	r[5], r[6], r[7] = 0, 0, 0                // 24-bit offset into the data section
	binary.BigEndian.PutUint32(r[8:], 0)      // reserved handle

	return fork, nil
}
