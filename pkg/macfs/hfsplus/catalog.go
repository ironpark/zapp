package hfsplus

import (
	"encoding/binary"
	"fmt"
	"sort"
	"time"
)

// Catalog record types, and the flag marking an entry that owns a thread record.
const (
	recordFolder       = 1
	recordFile         = 2
	recordFolderThread = 3
	recordFileThread   = 4

	flagThreadExists = 0x0002
)

// Sizes the format fixes for the records the catalog holds.
const (
	folderRecordSize = 88
	fileRecordSize   = 248
	forkDataSize     = 80
	maxCatalogKeyLen = 516
)

// hfsEpoch is 1904-01-01 UTC, the zero of the format's timestamps.
var hfsEpoch = time.Date(1904, time.January, 1, 0, 0, 0, 0, time.UTC)

// hfsTime converts a Go time to the format's seconds-since-1904, saturating
// rather than wrapping so that times outside the representable range stay
// ordered instead of jumping to the far end of it.
func hfsTime(t time.Time) uint32 {
	if t.IsZero() {
		return 0
	}
	secs := t.Unix() - hfsEpoch.Unix()
	if secs < 0 {
		return 0
	}
	if secs > 0xFFFFFFFF {
		return 0xFFFFFFFF
	}
	return uint32(secs)
}

// catalogKey is a parent ID paired with a name, the key the catalog is sorted by.
type catalogKey struct {
	parentID uint32
	name     []uint16
}

func (k catalogKey) encode() []byte {
	// The length field covers everything after itself.
	length := 6 + 2*len(k.name)
	b := make([]byte, 2+length)
	binary.BigEndian.PutUint16(b, uint16(length))
	binary.BigEndian.PutUint32(b[2:], k.parentID)
	binary.BigEndian.PutUint16(b[6:], uint16(len(k.name)))
	for i, u := range k.name {
		binary.BigEndian.PutUint16(b[8+2*i:], u)
	}
	return b
}

// compare orders keys by parent first and name second, as the B-tree requires.
func (k catalogKey) compare(other catalogKey) int {
	if k.parentID != other.parentID {
		if k.parentID < other.parentID {
			return -1
		}
		return 1
	}
	return compareNames(k.name, other.name)
}

// sortChildren returns dir entries in catalog order, rejecting names that
// collide under the case-folded comparison the catalog sorts by.
func sortChildren(children []*Node) ([]*Node, error) {
	type named struct {
		node *Node
		name []uint16
	}
	entries := make([]named, len(children))
	for i, child := range children {
		name, err := encodeName(child.Name)
		if err != nil {
			return nil, err
		}
		entries[i] = named{child, name}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return compareNames(entries[i].name, entries[j].name) < 0
	})
	out := make([]*Node, len(entries))
	for i, e := range entries {
		if i > 0 && compareNames(entries[i-1].name, e.name) == 0 {
			return nil, fmt.Errorf("duplicate file name in one directory: %q", e.node.Name)
		}
		out[i] = e.node
	}
	return out, nil
}
