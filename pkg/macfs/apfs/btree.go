package apfs

import (
	"context"
	"encoding/binary"
	"fmt"
)

const (
	blockSize   = 4096
	physical    = uint32(0x40000000)
	ephemeral   = uint32(0x80000000)
	transaction = uint64(1)
)

var le = binary.LittleEndian

func put16(b []byte, off int, v uint16) { le.PutUint16(b[off:], v) }
func put32(b []byte, off int, v uint32) { le.PutUint32(b[off:], v) }
func put64(b []byte, off int, v uint64) { le.PutUint64(b[off:], v) }
func u64(v uint64) []byte               { return le.AppendUint64(nil, v) }

// APFS stores the two check words that make the Fletcher sums vanish, rather
// than storing the running sums themselves. The checksum excludes its 8 bytes.
func checksum(b []byte) uint64 {
	const mod = uint64(0xffffffff)
	var a, c uint64
	for off := 8; off < len(b); off += 4 {
		a = (a + uint64(le.Uint32(b[off:]))) % mod
		c = (c + a) % mod
	}
	x := mod - (a+c)%mod
	y := mod - (a+x)%mod
	return y<<32 | x
}
func seal(b []byte) { put64(b, 0, checksum(b)) }
func object(b []byte, oid uint64, typ, subtype uint32) {
	put64(b, 8, oid)
	put64(b, 16, transaction)
	put32(b, 24, typ)
	put32(b, 28, subtype)
}

type record struct{ key, value []byte }
type treeSpec struct {
	storage, subtype, flags uint32
	keySize, valueSize      int
}
type treeNode struct {
	records   []record
	children  []*treeNode
	level     uint16
	oid, addr uint64
}
type tree struct {
	spec                     treeSpec
	root                     *treeNode
	nodes                    []*treeNode
	count                    uint64
	longestKey, longestValue uint32
}

// Trees are packed bottom-up. Every level reserves a root footer's worth of
// space, so promotion to a root never forces an otherwise unnecessary split.
func buildTree(ctx context.Context, spec treeSpec, records []record) (*tree, error) {
	t := &tree{spec: spec, count: uint64(len(records))}
	for _, r := range records {
		t.longestKey = max(t.longestKey, uint32(len(r.key)))
		t.longestValue = max(t.longestValue, uint32(len(r.value)))
	}
	level := uint16(0)
	var children []*treeNode
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		tocSize := 8
		if spec.keySize != 0 && (level > 0 || spec.valueSize != 0) {
			tocSize = 4
		}
		var nodes []*treeNode
		for start := 0; start < len(records) || len(nodes) == 0; {
			end, used := start, 56+40
			if tocSize == 4 {
				used += fixedTableSize(spec, level)
			}
			for end < len(records) {
				r := records[end]
				cost := len(r.key) + len(r.value)
				if tocSize != 4 && (end-start)%8 == 0 {
					cost += 8 * tocSize
				}
				if used+cost > blockSize {
					break
				}
				used += cost
				end++
			}
			if end == start && start < len(records) {
				return nil, fmt.Errorf("APFS B-tree record is too large")
			}
			n := &treeNode{records: records[start:end], level: level}
			if level > 0 {
				n.children = children[start:end]
			}
			nodes = append(nodes, n)
			t.nodes = append(t.nodes, n)
			start = end
		}
		if len(nodes) == 1 {
			t.root = nodes[0]
			return t, nil
		}
		if level > 0 && len(nodes) >= len(children) {
			return nil, fmt.Errorf("APFS B-tree keys are too large to branch")
		}
		children = nodes
		records = make([]record, len(nodes))
		for i, n := range nodes {
			records[i] = record{n.records[0].key, make([]byte, 8)}
		}
		level++
	}
}

func (t *tree) place(next *uint64, nextOID *uint64) {
	for _, n := range t.nodes {
		n.addr = *next
		*next++
		if t.spec.storage == physical {
			n.oid = n.addr
		} else {
			n.oid = *nextOID
			*nextOID++
		}
	}
}

func (t *tree) encode(n *treeNode) []byte {
	b := make([]byte, blockSize)
	flags := uint16(0)
	typ := uint32(3)
	if n == t.root {
		flags |= 1
		typ = 2
	}
	if n.level == 0 {
		flags |= 2
	}
	tocSize := 8
	if t.spec.keySize != 0 && (n.level > 0 || t.spec.valueSize != 0) {
		flags |= 4
		tocSize = 4
	}
	object(b, n.oid, t.spec.storage|typ, t.spec.subtype)
	put16(b, 32, flags)
	put16(b, 34, n.level)
	put32(b, 36, uint32(len(n.records)))
	tableLen := max(8, (len(n.records)+7)&^7) * tocSize
	if tocSize == 4 {
		tableLen = fixedTableSize(t.spec, n.level)
	}
	put16(b, 42, uint16(tableLen))
	keyStart := 56 + tableLen
	valueEnd := blockSize
	if n == t.root {
		valueEnd -= 40
	}
	ko, vo := 0, 0
	for i, r := range n.records {
		value := r.value
		if n.level > 0 {
			value = u64(n.children[i].oid)
		}
		vo += len(value)
		entry := 56 + i*tocSize
		put16(b, entry, uint16(ko))
		if tocSize == 4 {
			put16(b, entry+2, uint16(vo))
		} else {
			put16(b, entry+2, uint16(len(r.key)))
			put16(b, entry+4, uint16(vo))
			put16(b, entry+6, uint16(len(value)))
		}
		copy(b[keyStart+ko:], r.key)
		copy(b[valueEnd-vo:], value)
		ko += len(r.key)
	}
	put16(b, 44, uint16(ko))
	put16(b, 46, uint16(valueEnd-vo-keyStart-ko))
	put16(b, 48, 0xffff)
	put16(b, 52, 0xffff)
	if n == t.root {
		o := blockSize - 40
		put32(b, o, t.spec.flags)
		put32(b, o+4, blockSize)
		put32(b, o+8, uint32(t.spec.keySize))
		put32(b, o+12, uint32(t.spec.valueSize))
		put32(b, o+16, t.longestKey)
		put32(b, o+20, t.longestValue)
		put64(b, o+24, t.count)
		put64(b, o+32, uint64(len(t.nodes)))
	}
	seal(b)
	return b
}

func fixedTableSize(spec treeSpec, level uint16) int {
	valueSize := spec.valueSize
	if level > 0 {
		valueSize = 8
	}
	return ((blockSize-56-40)/(spec.keySize+valueSize+4) + 7) &^ 7 << 2
}
