package apfs

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"math"
	"sort"
)

type stream struct {
	id, owner, start, blocks uint64
	size                     int64
	source                   Source
}
type entry struct {
	node           *Node
	parent         uint64
	data, resource *stream
}

func (l *layout) collect(ctx context.Context) error {
	var nextID uint64 = 16
	var walk func(*Node, uint64) error
	walk = func(n *Node, parent uint64) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		nextID = max(nextID, n.ID+1)
		l.entries = append(l.entries, entry{node: n, parent: parent})
		for _, c := range n.Children {
			if err := walk(c, n.ID); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(l.volume.Root, 1); err != nil {
		return err
	}
	// APFS reserves inode 3 for its private directory outside the user root.
	l.entries = append(l.entries, entry{node: &Node{Name: "private-dir", Mode: fs.ModeDir | 0700, ID: 3}, parent: 1})
	add := func(id, owner uint64, src Source) (*stream, error) {
		if src == nil {
			return nil, nil
		}
		size := src.Size()
		if size < 0 || size > math.MaxInt64-blockSize {
			return nil, fmt.Errorf("invalid APFS source size %d", size)
		}
		s := &stream{id: id, owner: owner, size: size, blocks: blocksFor(uint64(size)), source: src}
		l.streams = append(l.streams, s)
		return s, nil
	}
	for i := range l.entries {
		e := &l.entries[i]
		n := e.node
		var err error
		if n.IsDir() {
			if n.ID >= 16 {
				l.directories++
			}
		} else if n.IsSymlink() {
			l.symlinks++
		} else {
			l.files++
			e.data, err = add(n.ID, n.ID, n.Data)
			if err != nil {
				return err
			}
		}
		if n.ResourceFork != nil {
			e.resource, err = add(nextID, n.ID, n.ResourceFork)
			nextID++
			if err != nil {
				return err
			}
		}
	}
	l.nextFileID = nextID
	return nil
}

func fskey(id uint64, kind uint64) []byte { return u64(id | kind<<60) }

// Filesystem keys order the object ID first and record type second, unlike
// comparing the packed 64-bit header as an integer.
func (l *layout) compareKey(a, b []byte) int {
	x, y := le.Uint64(a), le.Uint64(b)
	if i, j := x&0x0fffffffffffffff, y&0x0fffffffffffffff; i < j {
		return -1
	} else if i > j {
		return 1
	}
	if x>>60 < y>>60 {
		return -1
	}
	if x>>60 > y>>60 {
		return 1
	}
	switch x >> 60 {
	case 9:
		i, j := le.Uint32(a[8:])>>10, le.Uint32(b[8:])>>10
		if i < j {
			return -1
		}
		if i > j {
			return 1
		}
		return bytes.Compare([]byte(normalizedName(string(a[12:len(a)-1]), !l.volume.CaseSensitive)), []byte(normalizedName(string(b[12:len(b)-1]), !l.volume.CaseSensitive)))
	case 4:
		return bytes.Compare(a[10:], b[10:])
	case 8:
		i, j := le.Uint64(a[8:]), le.Uint64(b[8:])
		if i < j {
			return -1
		}
		if i > j {
			return 1
		}
		return 0
	}
	return bytes.Compare(a[8:], b[8:])
}

type xfield struct {
	kind, flags byte
	data        []byte
}

func xfields(fields []xfield) []byte {
	if len(fields) == 0 {
		return nil
	}
	n := 0
	for _, f := range fields {
		n += (len(f.data) + 7) &^ 7
	}
	b := make([]byte, 4+4*len(fields)+n)
	put16(b, 0, uint16(len(fields)))
	put16(b, 2, uint16(n))
	o := 4 + 4*len(fields)
	for i, f := range fields {
		b[4+4*i] = f.kind
		b[5+4*i] = f.flags
		put16(b, 6+4*i, uint16(len(f.data)))
		copy(b[o:], f.data)
		o += (len(f.data) + 7) &^ 7
	}
	return b
}

func dstream(s *stream) []byte {
	b := make([]byte, 40)
	put64(b, 0, uint64(s.size))
	put64(b, 8, s.blocks*blockSize)
	put64(b, 24, uint64(s.size))
	return b
}

func (l *layout) catalog() []record {
	var records []record
	add := func(k, v []byte) { records = append(records, record{k, v}) }
	stamp := uint64(l.volume.Created.UnixNano())
	for _, e := range l.entries {
		n := e.node
		name := n.Name
		if n.ID == 2 {
			name = "root"
		}
		mod := stamp
		if !n.ModTime.IsZero() {
			mod = uint64(n.ModTime.UnixNano())
		}
		b := make([]byte, 92)
		put64(b, 0, e.parent)
		put64(b, 8, n.ID)
		put64(b, 16, stamp)
		put64(b, 24, mod)
		put64(b, 32, mod)
		put64(b, 40, mod)
		put64(b, 48, 0x8000) // INODE_NO_RSRC_FORK, cleared below when present.
		if e.resource != nil {
			put64(b, 48, 0x4000) // INODE_HAS_RSRC_FORK
		}
		if n.FinderInfo != [32]byte{} {
			put64(b, 48, le.Uint64(b[48:])|0x100)
		}
		put32(b, 56, 1)
		put32(b, 64, 1)
		mode, kind := uint16(0100000|n.Mode.Perm()), uint16(8)
		if n.IsDir() {
			mode = 0040000 | uint16(n.Mode.Perm())
			kind = 4
			put32(b, 56, uint32(len(n.Children)))
			if n.Mode.Perm() == 0 {
				mode |= 0755
			}
		} else if n.IsSymlink() {
			mode = 0120000 | 0755
			kind = 10
		}
		if n.Mode&fs.ModeSetuid != 0 {
			mode |= 04000
		}
		if n.Mode&fs.ModeSetgid != 0 {
			mode |= 02000
		}
		if n.Mode&fs.ModeSticky != 0 {
			mode |= 01000
		}
		put16(b, 80, mode)
		fields := []xfield{{4, 2, append([]byte(name), 0)}}
		if e.data != nil {
			fields = append(fields, xfield{8, 1, dstream(e.data)})
		}
		b = append(b, xfields(fields)...)
		add(fskey(n.ID, 3), b)
		key := fskey(e.parent, 9)
		key = le.AppendUint32(key, nameHash(name, !l.volume.CaseSensitive)<<10|uint32(len(name)+1))
		key = append(append(key, []byte(name)...), 0)
		v := make([]byte, 18)
		put64(v, 0, n.ID)
		put64(v, 8, stamp)
		put16(v, 16, kind)
		add(key, v)
		attr := func(name string, flags uint16, data []byte) {
			k := le.AppendUint16(fskey(n.ID, 4), uint16(len(name)+1))
			k = append(append(k, []byte(name)...), 0)
			v := le.AppendUint16(nil, flags)
			v = le.AppendUint16(v, uint16(len(data)))
			v = append(v, data...)
			add(k, v)
		}
		if n.IsSymlink() {
			attr("com.apple.fs.symlink", 2, append([]byte(n.LinkTarget), 0))
		}
		if n.FinderInfo != [32]byte{} {
			attr("com.apple.FinderInfo", 2, n.FinderInfo[:])
		}
		if e.resource != nil {
			attr("com.apple.ResourceFork", 1, append(u64(e.resource.id), dstream(e.resource)...))
		}
	}
	for _, s := range l.streams {
		add(fskey(s.id, 6), le.AppendUint32(nil, 1))
		if s.blocks == 0 {
			continue
		}
		key := append(fskey(s.id, 8), u64(0)...)
		v := make([]byte, 24)
		put64(v, 0, s.blocks*blockSize)
		put64(v, 8, s.start)
		add(key, v)
	}
	sort.Slice(records, func(i, j int) bool { return l.compareKey(records[i].key, records[j].key) < 0 })
	return records
}

func (l *layout) extentRecords() []record {
	var r []record
	for _, s := range l.streams {
		if s.blocks == 0 {
			continue
		}
		v := make([]byte, 20)
		put64(v, 0, s.blocks|1<<60)
		put64(v, 8, s.owner)
		put32(v, 16, 1)
		r = append(r, record{fskey(s.start, 2), v})
	}
	return r
}
