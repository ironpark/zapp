package macpkg

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"path"
	"sort"
)

var be = binary.BigEndian

func u32(v uint32) []byte { b := make([]byte, 4); be.PutUint32(b, v); return b }
func words(v ...uint32) []byte {
	var b []byte
	for _, x := range v {
		b = append(b, u32(x)...)
	}
	return b
}

type bomStore struct{ blocks [][]byte }

func (s *bomStore) add(b []byte) uint32 {
	s.blocks = append(s.blocks, b)
	return uint32(len(s.blocks) - 1)
}

type bomPair struct{ value, key uint32 }

// node writes one 4096-byte B-tree block. Leaves carry the version word and the
// backward link that makes sequential lsbom traversal work; branches do not.
func (s *bomStore) node(chunk []bomPair, leaf bool, prev uint32) uint32 {
	b := make([]byte, 4096)
	if leaf {
		be.PutUint16(b, 1)
		be.PutUint32(b[8:], prev)
	}
	be.PutUint16(b[2:], uint16(len(chunk)))
	for i, p := range chunk {
		be.PutUint32(b[12+i*8:], p.value)
		be.PutUint32(b[16+i*8:], p.key)
	}
	return s.add(b)
}

// Each branch key is the last key of its child, as in BOM's B-tree. Leaves
// form a doubly linked list; both lookup and sequential lsbom traversal work.
func (s *bomStore) tree(pairs []bomPair) uint32 {
	const capacity = (4096 - 12) / 8
	var level []bomPair
	var prev uint32
	for start := 0; start < len(pairs) || start == 0; start += capacity {
		end := min(start+capacity, len(pairs))
		chunk := pairs[start:end]
		id := s.node(chunk, true, prev)
		if prev != 0 {
			be.PutUint32(s.blocks[prev][4:], id)
		}
		prev = id
		key := uint32(0)
		if len(chunk) > 0 {
			key = chunk[len(chunk)-1].key
		}
		level = append(level, bomPair{id, key})
	}
	for len(level) > 1 {
		var next []bomPair
		for start := 0; start < len(level); start += capacity {
			end := min(start+capacity, len(level))
			chunk := level[start:end]
			next = append(next, bomPair{s.node(chunk, false, 0), chunk[len(chunk)-1].key})
		}
		level = next
	}
	b := append([]byte("tree"), words(1, level[0].value, 4096, uint32(len(pairs)))...)
	b = append(b, 0)
	return s.add(b)
}

func makeBOM(entries []fileEntry) ([]byte, error) {
	s := bomStore{blocks: [][]byte{nil}}
	info := s.add(words(1, uint32(len(entries)+1), 1, 0, 0, 0, 0))
	type record struct {
		parent uint32
		name   string
		pair   bomPair
	}
	var records []record
	var sizes []bomPair
	pathIDs := map[string]uint32{"": 0}
	for i, e := range entries {
		id := uint32(i + 1)
		parent := uint32(0)
		name := "."
		if e.name != "." {
			parent = pathIDs[path.Dir(e.name)]
			name = path.Base(e.name)
		}
		pathIDs[e.name] = id
		typ := byte(1)
		if e.dir() {
			typ = 2
		} else if e.link != "" {
			typ = 3
		}
		b := make([]byte, 31)
		b[0] = typ
		b[1] = 1
		be.PutUint16(b[2:], 15)
		be.PutUint16(b[4:], uint16(e.mode))
		be.PutUint32(b[6:], e.uid)
		be.PutUint32(b[10:], e.gid)
		be.PutUint32(b[14:], e.mtime)
		be.PutUint32(b[18:], uint32(e.size))
		b[22] = 1
		be.PutUint32(b[23:], e.checksum)
		if typ == 3 {
			be.PutUint32(b[27:], uint32(len(e.link)+1))
			b = append(b, []byte(e.link)...)
			b = append(b, 0)
		}
		if typ != 2 {
			b = append(b, 0, 0, 0, 0)
		}
		meta := s.add(b)
		if e.size > math.MaxUint32 {
			value := make([]byte, 8)
			be.PutUint64(value, uint64(e.size))
			sizes = append(sizes, bomPair{s.add(value), s.add(u32(meta))})
		}
		key := s.add(append(append(u32(parent), []byte(name)...), 0))
		value := s.add(words(id, meta))
		records = append(records, record{parent, name, bomPair{value, key}})
	}
	sort.Slice(records, func(i, j int) bool {
		a, b := records[i], records[j]
		if a.parent != b.parent {
			return a.parent < b.parent
		}
		return a.name < b.name
	})
	pairs := make([]bomPair, len(records))
	for i, r := range records {
		pairs[i] = r.pair
	}
	paths := s.tree(pairs)
	hl := s.tree(nil)
	vt := s.tree(nil)
	vi := s.add(append(words(1, vt, 0), 0))
	sizeTree := s.tree(sizes)
	vars := words(5)
	for _, v := range []struct {
		name string
		id   uint32
	}{{"BomInfo", info}, {"Paths", paths}, {"HLIndex", hl}, {"VIndex", vi}, {"Size64", sizeTree}} {
		vars = append(vars, u32(v.id)...)
		vars = append(vars, byte(len(v.name)))
		vars = append(vars, v.name...)
	}
	var out bytes.Buffer
	out.Write(make([]byte, 512))
	offsets := make([]uint32, len(s.blocks))
	for i, b := range s.blocks {
		if i == 0 {
			continue
		}
		if uint64(out.Len())+uint64(len(b)) > math.MaxUint32 {
			return nil, fmt.Errorf("BOM exceeds 32-bit offsets")
		}
		offsets[i] = uint32(out.Len())
		out.Write(b)
	}
	vo := uint32(out.Len())
	out.Write(vars)
	ioff := uint32(out.Len())
	out.Write(u32(uint32(len(s.blocks))))
	for i, b := range s.blocks {
		out.Write(words(offsets[i], uint32(len(b))))
	}
	out.Write(words(0))
	b := out.Bytes()
	copy(b, "BOMStore")
	copy(b[8:], words(1, uint32(len(s.blocks)-1), ioff, uint32(len(b))-ioff, vo, uint32(len(vars))))
	return b, nil
}
