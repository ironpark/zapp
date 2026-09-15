// alias/create.go
package alias

import (
	"encoding/binary"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

func utf16be(str string) []byte {
	u16 := utf16.Encode([]rune(str))
	b := make([]byte, len(u16)*2)
	for i, v := range u16 {
		binary.BigEndian.PutUint16(b[i*2:], v)
	}
	return b
}

// Target describes the file an alias record should point at. Every field is
// supplied by the caller rather than read from a live filesystem, so a record
// can be built for a volume that has not been created yet, which is how a disk
// image can carry an alias into a volume it is still being assembled from.
type Target struct {
	// Path is where the target sits within its volume, as an absolute POSIX
	// path from the volume root, such as "/.background/background.png".
	Path string

	// ID is the target's catalog node ID, which a mounted volume reports as its
	// inode number, and ParentID is the same for the directory holding it.
	ID       uint32
	ParentID uint32

	IsDir   bool
	Created time.Time

	// VolumeName is the name the Finder shows for the volume, and the one the
	// record stores.
	VolumeName    string
	VolumeCreated time.Time

	// Identity describes the filesystem holding the target. Its zero value
	// gives the HFS+ record this package produced before other filesystems
	// were supported.
	Identity VolumeIdentity
}

// VolumeIdentity is the legacy filesystem description an alias record carries,
// as FSNewAlias reports it for a mounted volume.
type VolumeIdentity struct {
	Signature  string
	FSID       uint16
	Attributes uint32
	Type       string
}

// Create encodes an alias record pointing at t.
func Create(t Target) ([]byte, error) {
	if t.VolumeName == "" {
		return nil, fmt.Errorf("volume name is required")
	}
	if !path.IsAbs(t.Path) || path.Clean(t.Path) != t.Path || t.Path == "/" {
		return nil, fmt.Errorf("target path must be a clean absolute path within the volume: %q", t.Path)
	}
	name := path.Base(t.Path)
	parentName := path.Base(path.Dir(t.Path))
	if parentName == "/" {
		// The volume root is named after the volume rather than "/".
		parentName = t.VolumeName
	}

	info := Info{Version: 2, Extra: []Extra{}}
	info.Target.ID = t.ID
	info.Target.Type = "file"
	if t.IsDir {
		info.Target.Type = "directory"
	}
	info.Target.Filename = legacyName(name, 63)
	info.Target.Created = t.Created

	info.Parent.ID = t.ParentID
	info.Parent.Name = parentName

	info.Volume.Name = legacyName(t.VolumeName, 27)
	info.Volume.Created = t.VolumeCreated
	info.Volume.Signature = "H+"
	if t.Identity.Signature != "" {
		info.Volume.Signature = t.Identity.Signature
	}
	info.Volume.FSID = t.Identity.FSID
	info.Volume.Attributes = t.Identity.Attributes
	if info.Volume.Attributes == 0 {
		info.Volume.Attributes = 0x00000D02
	}
	info.Volume.Type = "other"
	if t.Identity.Type != "" {
		info.Volume.Type = t.Identity.Type
	}

	// The record repeats the names and identifiers it already carries as a list
	// of tagged extras, which is what modern readers actually look at.
	add := func(kind int16, data []byte) {
		info.Extra = append(info.Extra, Extra{Type: kind, Length: uint16(len(data)), Data: data})
	}
	add(0, []byte(parentName))
	add(1, binary.BigEndian.AppendUint32(nil, t.ParentID))
	add(14, prefixedUTF16(name))
	add(15, prefixedUTF16(t.VolumeName))
	add(18, []byte(t.Path))
	// The mount point of a volume other than the boot volume.
	add(19, []byte("/Volumes/"+strings.ReplaceAll(t.VolumeName, "/", ":")))

	return Encode(info)
}

// Full names live in the Unicode extras. The legacy Pascal-string slots only
// hold a short fallback and must not reject otherwise valid APFS volume names.
func legacyName(name string, limit int) string {
	if len(name) <= limit {
		return name
	}
	for !utf8.RuneStart(name[limit]) {
		limit--
	}
	return name[:limit]
}

// prefixedUTF16 encodes s as the length-prefixed UTF-16 the extras use, where
// the length counts characters rather than bytes.
func prefixedUTF16(s string) []byte {
	encoded := utf16be(s)
	return append(binary.BigEndian.AppendUint16(nil, uint16(len(encoded)/2)), encoded...)
}
