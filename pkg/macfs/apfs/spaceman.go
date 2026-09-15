package apfs

import (
	"context"
	"fmt"
)

// spaceManager writes a clean initial checkpoint. The internal pool owns the
// chunk bitmaps and CIB/CAB blocks; the main allocator reserves the entire
// pool, checkpoint areas and internal-pool bitmap ring.
func (l *layout) spaceManager(ctx context.Context) error {
	const headerSize = 2520 // Version 1 spaceman, including allocation zones.
	align8 := func(n uint64) uint64 { return (n + 7) &^ 7 }
	xidOffset := uint64(headerSize)
	bitmapOffset := xidOffset + 8*l.ipBitmapBlocks
	nextOffset := bitmapOffset + align8(2*l.ipBitmapBlocks)
	addrOffset := nextOffset + align8(2*16*l.ipBitmapBlocks)
	addressCount := l.cibs
	if l.cabs != 0 {
		addressCount = l.cabs
	}
	smBlocks := blocksFor(addrOffset + 8*addressCount)
	if smBlocks+3 > l.checkpointBlocks/4 {
		return fmt.Errorf("APFS space manager exceeds checkpoint capacity")
	}
	b := make([]byte, smBlocks*blockSize)
	object(b, spacemanOID, ephemeral|5, 0)
	put32(b, 32, blockSize)
	put32(b, 36, blocksPerChunk)
	put32(b, 40, chunksPerCIB)
	put32(b, 44, cibsPerCAB)
	put64(b, 48, l.total)
	put64(b, 56, l.chunks)
	put32(b, 64, uint32(l.cibs))
	put32(b, 68, uint32(l.cabs))
	put64(b, 72, l.total-l.used)
	put32(b, 80, uint32(addrOffset))
	put32(b, 128, uint32(addrOffset+8*addressCount))
	put32(b, 144, 1)
	put32(b, 148, 16)
	put64(b, 152, l.ipBlocks)
	put32(b, 160, uint32(l.ipBitmapBlocks))
	put32(b, 164, uint32(16*l.ipBitmapBlocks))
	put64(b, 168, l.ipBitmapBase)
	put64(b, 176, l.ipBase)
	put64(b, 208, ipQueueOID)
	put16(b, 224, l.ipQueueLimit)
	put64(b, 248, mainQueueOID)
	put16(b, 264, l.mainQueueLimit)
	// No free-queue entries: no earlier transactions have deferred frees.
	put16(b, 320, uint16(l.ipBitmapBlocks))
	put16(b, 322, uint16(16*l.ipBitmapBlocks-1))
	put32(b, 324, uint32(xidOffset))
	put32(b, 328, uint32(bitmapOffset))
	put32(b, 332, uint32(nextOffset))
	put32(b, 336, 1)
	put32(b, 340, headerSize)
	for i := uint64(0); i < l.ipBitmapBlocks; i++ {
		put64(b, int(xidOffset+8*i), transaction)
		put16(b, int(bitmapOffset+2*i), uint16(i))
	}
	for i := uint64(0); i < 16*l.ipBitmapBlocks; i++ {
		next := uint16(i + 1)
		if i < l.ipBitmapBlocks || i+1 == 16*l.ipBitmapBlocks {
			next = 0xffff
		}
		put16(b, int(nextOffset+2*i), next)
	}
	// Layout inside the internal pool: CIBs, CABs, then chunk bitmaps.
	bitmapBase := l.ipBase + l.cibs + l.cabs
	for i := uint64(0); i < l.cibs; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		addr := l.ipBase + i
		cib := make([]byte, blockSize)
		object(cib, addr, physical|7, 0)
		put32(cib, 32, uint32(i))
		count := min(uint64(chunksPerCIB), l.chunks-i*chunksPerCIB)
		put32(cib, 36, uint32(count))
		for j := uint64(0); j < count; j++ {
			chunk := i*chunksPerCIB + j
			start := chunk * blocksPerChunk
			blocks := min(uint64(blocksPerChunk), l.total-start)
			used := uint64(0)
			if l.used > start {
				used = min(blocks, l.used-start)
			}
			o := int(40 + j*32)
			put64(cib, o, transaction)
			put64(cib, o+8, start)
			put32(cib, o+16, uint32(blocks))
			put32(cib, o+20, uint32(blocks-used))
			put64(cib, o+24, bitmapBase+chunk)
			bm := make([]byte, blockSize)
			setBits(bm, 0, used)
			l.blocks[bitmapBase+chunk] = bm
		}
		seal(cib)
		l.blocks[addr] = cib
		if l.cabs == 0 {
			put64(b, int(addrOffset+8*i), addr)
		}
	}
	for i := uint64(0); i < l.cabs; i++ {
		addr := l.ipBase + l.cibs + i
		cab := make([]byte, blockSize)
		object(cab, addr, physical|6, 0)
		put32(cab, 32, uint32(i))
		count := min(uint64(cibsPerCAB), l.cibs-i*cibsPerCAB)
		put32(cab, 36, uint32(count))
		for j := uint64(0); j < count; j++ {
			put64(cab, int(40+j*8), l.ipBase+i*cibsPerCAB+j)
		}
		seal(cab)
		l.blocks[addr] = cab
		put64(b, int(addrOffset+8*i), addr)
	}
	seal(b)
	l.blocks[dataBase] = b
	allocated := l.chunks + l.cibs + l.cabs
	for i := uint64(0); i < l.ipBitmapBlocks; i++ {
		bm := make([]byte, blockSize)
		start := i * blocksPerChunk
		if allocated > start {
			setBits(bm, 0, min(uint64(blocksPerChunk), allocated-start))
		}
		l.blocks[l.ipBitmapBase+i] = bm
	}
	reaperAddr := uint64(dataBase) + smBlocks
	r := make([]byte, blockSize)
	object(r, reaperOID, ephemeral|17, 0)
	put64(r, 32, 1)
	put32(r, 64, 1)
	put32(r, 108, blockSize-112)
	seal(r)
	l.blocks[reaperAddr] = r
	for i, oid := range []uint64{ipQueueOID, mainQueueOID} {
		t, err := buildTree(ctx, treeSpec{storage: ephemeral, subtype: 9, flags: 0x0e, keySize: 16, valueSize: 8}, nil)
		if err != nil {
			return err
		}
		t.root.oid = oid
		l.blocks[reaperAddr+1+uint64(i)] = t.encode(t.root)
	}
	// Map each ephemeral object to its physical checkpoint data extent.
	cp := make([]byte, blockSize)
	object(cp, 1, physical|12, 0)
	put32(cp, 32, 1)
	put32(cp, 36, 4)
	for i, m := range []struct {
		oid, addr, size uint64
		typ, sub        uint32
	}{
		{spacemanOID, dataBase, smBlocks * blockSize, ephemeral | 5, 0},
		{reaperOID, reaperAddr, blockSize, ephemeral | 17, 0},
		{ipQueueOID, reaperAddr + 1, blockSize, ephemeral | 2, 9},
		{mainQueueOID, reaperAddr + 2, blockSize, ephemeral | 2, 9},
	} {
		o := 40 + i*40
		put32(cp, o, m.typ)
		put32(cp, o+4, m.sub)
		put32(cp, o+8, uint32(m.size))
		put64(cp, o+24, m.oid)
		put64(cp, o+32, m.addr)
	}
	seal(cp)
	l.blocks[1] = cp
	return nil
}

func setBits(b []byte, from, to uint64) {
	for i := from; i < to; i++ {
		b[i/8] |= 1 << (i % 8)
	}
}

// A tree growing beyond one leaf also needs its root, so a two-node capacity
// must allow three nodes. These limits are part of macOS's spaceman sanity
// checks even for a read-only image whose free queues are empty.
func queueLimit(nodes uint64) uint16 {
	if nodes == 2 {
		nodes = 3
	}
	return uint16(min(nodes, uint64(0xffff)))
}
