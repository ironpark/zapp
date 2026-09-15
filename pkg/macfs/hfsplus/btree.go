package hfsplus

import (
	"encoding/binary"
	"fmt"
)

// B-tree node kinds, as stored in the node descriptor.
const (
	kindLeaf   = 0xFF // -1 as a signed byte.
	kindIndex  = 0
	kindHeader = 1
)

// B-tree header attributes and key comparison rules.
const (
	attrBigKeys          = 0x00000002
	attrVariableIndexKey = 0x00000004

	compareCaseFolding = 0xCF

	nodeDescriptorSize = 14
	headerRecordSize   = 106
	headerUserDataSize = 128
)

// btreeRecord is one leaf entry: a key encoded with its own length prefix,
// followed by the record body.
type btreeRecord struct {
	key  []byte
	data []byte
}

// btreeSpec describes a tree to build. Trees here are bulk loaded from records
// that are already in key order, so nodes are filled to capacity and no
// splitting or rebalancing is needed.
type btreeSpec struct {
	nodeSize       uint16
	maxKeyLength   uint16
	keyCompareType uint8
	attributes     uint32
}

// buildBTree lays out a whole B-tree file. records must already be sorted by
// key. An empty tree is legal and yields a header node alone.
func buildBTree(spec btreeSpec, records []btreeRecord) ([]byte, error) {
	nodeSize := int(spec.nodeSize)
	// Every record has to fit a node alongside the descriptor and the two
	// offsets that bracket it.
	capacity := nodeSize - nodeDescriptorSize - 4
	for _, r := range records {
		if len(r.key)+len(r.data) > capacity {
			return nil, fmt.Errorf("b-tree record of %d bytes exceeds the %d byte node capacity", len(r.key)+len(r.data), capacity)
		}
	}

	// Pack the leaves, remembering the first key of each so the level above can
	// point at it.
	var leaves [][]btreeRecord
	var firstKeys [][]byte
	used := 0
	for _, r := range records {
		size := len(r.key) + len(r.data)
		if len(leaves) == 0 || used+size+2 > nodeSize-nodeDescriptorSize-2 {
			leaves = append(leaves, nil)
			firstKeys = append(firstKeys, r.key)
			used = 0
		}
		leaves[len(leaves)-1] = append(leaves[len(leaves)-1], r)
		used += size + 2
	}

	nodes := make([][]byte, 1, len(leaves)+8) // Node 0 is the header.
	var rootNode, firstLeaf, lastLeaf uint32
	depth := 0

	if len(leaves) > 0 {
		firstLeaf, lastLeaf = 1, uint32(len(leaves))
		for i, leaf := range leaves {
			number := uint32(i + 1)
			var next, prev uint32
			if number > firstLeaf {
				prev = number - 1
			}
			if number < lastLeaf {
				next = number + 1
			}
			node, err := encodeNode(nodeSize, kindLeaf, 1, next, prev, leaf)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		}
		depth = 1
		rootNode = firstLeaf

		// Stack index levels until one node covers the level below it.
		level := make([]uint32, len(leaves))
		for i := range level {
			level[i] = uint32(i + 1)
		}
		keys := firstKeys
		for len(level) > 1 {
			depth++
			var nextLevel []uint32
			var nextKeys [][]byte
			var current []btreeRecord
			var currentUsed int
			flush := func() error {
				if len(current) == 0 {
					return nil
				}
				number := uint32(len(nodes))
				var prev uint32
				if len(nextLevel) > 0 {
					prev = nextLevel[len(nextLevel)-1]
					// Link the sibling that was written just before this one.
					binary.BigEndian.PutUint32(nodes[prev], number)
				}
				node, err := encodeNode(nodeSize, kindIndex, uint8(depth), 0, prev, current)
				if err != nil {
					return err
				}
				nodes = append(nodes, node)
				nextLevel = append(nextLevel, number)
				current, currentUsed = nil, 0
				return nil
			}
			for i, child := range level {
				pointer := make([]byte, 4)
				binary.BigEndian.PutUint32(pointer, child)
				record := btreeRecord{key: keys[i], data: pointer}
				size := len(record.key) + len(record.data)
				if currentUsed+size+2 > nodeSize-nodeDescriptorSize-2 {
					if err := flush(); err != nil {
						return nil, err
					}
				}
				if len(current) == 0 {
					nextKeys = append(nextKeys, keys[i])
				}
				current = append(current, record)
				currentUsed += size + 2
			}
			if err := flush(); err != nil {
				return nil, err
			}
			level, keys = nextLevel, nextKeys
		}
		rootNode = level[0]
	}

	total := uint32(len(nodes))
	header, err := encodeHeaderNode(spec, headerFields{
		treeDepth:     uint16(depth),
		rootNode:      rootNode,
		leafRecords:   uint32(len(records)),
		firstLeafNode: firstLeaf,
		lastLeafNode:  lastLeaf,
		totalNodes:    total,
	})
	if err != nil {
		return nil, err
	}
	nodes[0] = header

	out := make([]byte, 0, len(nodes)*nodeSize)
	for _, node := range nodes {
		out = append(out, node...)
	}
	return out, nil
}

// encodeNode writes one node: a descriptor, the records packed forwards from
// the end of it, and the offset of each record packed backwards from the end of
// the node.
func encodeNode(nodeSize int, kind uint8, height uint8, fLink, bLink uint32, records []btreeRecord) ([]byte, error) {
	node := make([]byte, nodeSize)
	binary.BigEndian.PutUint32(node, fLink)
	binary.BigEndian.PutUint32(node[4:], bLink)
	node[8] = kind
	node[9] = height
	binary.BigEndian.PutUint16(node[10:], uint16(len(records)))

	offset := nodeDescriptorSize
	for i, r := range records {
		if offset+len(r.key)+len(r.data) > nodeSize-2*(len(records)+1) {
			return nil, fmt.Errorf("b-tree node overflowed while packing record %d", i)
		}
		binary.BigEndian.PutUint16(node[nodeSize-2*(i+1):], uint16(offset))
		offset += copy(node[offset:], r.key)
		offset += copy(node[offset:], r.data)
	}
	binary.BigEndian.PutUint16(node[nodeSize-2*(len(records)+1):], uint16(offset))
	return node, nil
}

type headerFields struct {
	treeDepth     uint16
	rootNode      uint32
	leafRecords   uint32
	firstLeafNode uint32
	lastLeafNode  uint32
	totalNodes    uint32
}

// encodeHeaderNode writes node 0, which carries the tree's parameters and the
// bitmap of which nodes are in use.
func encodeHeaderNode(spec btreeSpec, f headerFields) ([]byte, error) {
	nodeSize := int(spec.nodeSize)
	mapSize := nodeSize - nodeDescriptorSize - headerRecordSize - headerUserDataSize - 8
	if int(f.totalNodes) > mapSize*8 {
		return nil, fmt.Errorf("b-tree of %d nodes needs a map node, which is not supported", f.totalNodes)
	}

	header := make([]byte, headerRecordSize)
	binary.BigEndian.PutUint16(header, f.treeDepth)
	binary.BigEndian.PutUint32(header[2:], f.rootNode)
	binary.BigEndian.PutUint32(header[6:], f.leafRecords)
	binary.BigEndian.PutUint32(header[10:], f.firstLeafNode)
	binary.BigEndian.PutUint32(header[14:], f.lastLeafNode)
	binary.BigEndian.PutUint16(header[18:], spec.nodeSize)
	binary.BigEndian.PutUint16(header[20:], spec.maxKeyLength)
	binary.BigEndian.PutUint32(header[22:], f.totalNodes)
	binary.BigEndian.PutUint32(header[26:], 0) // No free nodes: the file is sized to fit.
	binary.BigEndian.PutUint32(header[32:], uint32(nodeSize))
	header[36] = 0 // A HFS B-tree rather than a user one.
	header[37] = spec.keyCompareType
	binary.BigEndian.PutUint32(header[38:], spec.attributes)

	// The map marks every node the tree actually uses, most significant bit first.
	bitmap := make([]byte, mapSize)
	for i := uint32(0); i < f.totalNodes; i++ {
		bitmap[i/8] |= 0x80 >> (i % 8)
	}

	return encodeNode(nodeSize, kindHeader, 0, 0, 0, []btreeRecord{
		{data: header},
		{data: make([]byte, headerUserDataSize)},
		{data: bitmap},
	})
}
