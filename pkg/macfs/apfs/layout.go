package apfs

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"sort"
)

const (
	descBlocks     = 8
	dataBase       = 1 + descBlocks
	spacemanOID    = 1024
	reaperOID      = 1025
	volumeOID      = 1026
	ipQueueOID     = 1027
	mainQueueOID   = 1028
	firstTreeOID   = 1029
	blocksPerChunk = blockSize * 8
	chunksPerCIB   = (blockSize - 40) / 32
	cibsPerCAB     = (blockSize - 40) / 8
)

type layout struct {
	volume                                                             Volume
	entries                                                            []entry
	streams                                                            []*stream
	total, used, nextFileID, nextOID                                   uint64
	files, directories, symlinks                                       uint64
	chunks, cibs, cabs, ipBlocks, ipBitmapBlocks, ipBase, ipBitmapBase uint64
	containerMap, volumeMap, volumeBlock, volumeAllocated              uint64
	fsTree, extentTree, snapTree, volumeMapTree, containerMapTree      *tree
	blocks                                                             map[uint64][]byte
	checkpointBlocks                                                   uint64
	ipQueueLimit, mainQueueLimit                                       uint16
}

func blocksFor(n uint64) uint64 { return (n + blockSize - 1) / blockSize }

func planVolume(ctx context.Context, v Volume) (*layout, error) {
	l := &layout{volume: v, blocks: make(map[uint64][]byte)}
	if err := l.collect(ctx); err != nil {
		return nil, err
	}
	var dataSize uint64
	for _, s := range l.streams {
		if s.blocks > math.MaxInt64/blockSize-dataSize {
			return nil, fmt.Errorf("APFS image is too large")
		}
		dataSize += s.blocks
	}
	var err error
	l.fsTree, err = buildTree(ctx, treeSpec{subtype: 14, flags: 0x42}, l.catalog())
	if err != nil {
		return nil, err
	}
	l.extentTree, err = buildTree(ctx, treeSpec{storage: physical, subtype: 15, flags: 0x52}, l.extentRecords())
	if err != nil {
		return nil, err
	}
	l.snapTree, err = buildTree(ctx, treeSpec{storage: physical, subtype: 16, flags: 0x52}, nil)
	if err != nil {
		return nil, err
	}
	// The object maps' size depends only on the number of virtual objects.
	mapRecords := make([]record, len(l.fsTree.nodes))
	for i := range mapRecords {
		mapRecords[i] = record{make([]byte, 16), make([]byte, 16)}
	}
	omapSpec := treeSpec{storage: physical, subtype: 11, flags: 0x12, keySize: 16, valueSize: 16}
	l.volumeMapTree, err = buildTree(ctx, omapSpec, mapRecords)
	if err != nil {
		return nil, err
	}
	l.containerMapTree, err = buildTree(ctx, omapSpec, mapRecords[:1])
	if err != nil {
		return nil, err
	}
	metadata := uint64(3 + len(l.fsTree.nodes) + len(l.extentTree.nodes) + len(l.snapTree.nodes) + len(l.volumeMapTree.nodes) + len(l.containerMapTree.nodes))
	l.total = max(uint64(8192), dataSize+metadata+1024)
	// The allocation bitmaps describe their own space. Iterate until sizing
	// their internal pool and all CIB/CAB levels no longer grows the container.
	for {
		l.chunks = (l.total + blocksPerChunk - 1) / blocksPerChunk
		l.cibs = (l.chunks + chunksPerCIB - 1) / chunksPerCIB
		l.cabs = 0
		if l.cibs > 128 {
			l.cabs = (l.cibs + cibsPerCAB - 1) / cibsPerCAB
		}
		l.ipBlocks = 3 * (l.chunks + l.cibs + l.cabs)
		l.ipBitmapBlocks = (l.ipBlocks + blocksPerChunk - 1) / blocksPerChunk
		l.ipQueueLimit = queueLimit((3*l.chunks + 1126) / 1127)
		mainLimit := uint64(512)
		if l.total < 1<<18 {
			mainLimit = (l.total + 4543) / 4544
		} else if l.total < 1<<20 {
			mainLimit = (l.total + 2271) / 2272
		}
		l.mainQueueLimit = queueLimit(mainLimit)
		// Reserve four checkpoints, including room for the free queues to grow.
		l.checkpointBlocks = max(uint64(128), 4*(16+uint64(l.ipQueueLimit)+uint64(l.mainQueueLimit)))
		l.ipBitmapBase = dataBase + l.checkpointBlocks
		l.ipBase = l.ipBitmapBase + 16*l.ipBitmapBlocks
		needed := l.ipBase + l.ipBlocks + metadata + dataSize + 1024
		if needed <= l.total {
			break
		}
		l.total = needed
	}
	if l.total > math.MaxInt64/blockSize {
		return nil, fmt.Errorf("APFS image is too large")
	}
	if 16*l.ipBitmapBlocks > 0xfffe {
		return nil, fmt.Errorf("APFS internal pool exceeds bitmap limits")
	}
	next := l.ipBase + l.ipBlocks
	l.containerMap = next
	next++
	l.volumeBlock = next
	next++
	l.volumeMap = next
	next++
	l.nextOID = firstTreeOID
	for _, t := range []*tree{l.fsTree, l.extentTree, l.snapTree, l.volumeMapTree, l.containerMapTree} {
		t.place(&next, &l.nextOID)
	}
	for _, s := range l.streams {
		s.start = next
		next += s.blocks
	}
	l.used = next
	l.volumeAllocated = uint64(1+len(l.fsTree.nodes)+len(l.extentTree.nodes)+len(l.snapTree.nodes)+len(l.volumeMapTree.nodes)) + dataSize
	// Rebuild records with physical addresses, preserving the settled layout.
	fsFinal, err := buildTree(ctx, l.fsTree.spec, l.catalog())
	if err != nil {
		return nil, err
	}
	erFinal, err := buildTree(ctx, l.extentTree.spec, l.extentRecords())
	if err != nil {
		return nil, err
	}
	replace := func(old, updated *tree) error {
		if len(old.nodes) != len(updated.nodes) {
			return fmt.Errorf("APFS tree changed size during layout")
		}
		for i, n := range updated.nodes {
			n.oid = old.nodes[i].oid
			n.addr = old.nodes[i].addr
		}
		*old = *updated
		return nil
	}
	if err = replace(l.fsTree, fsFinal); err != nil {
		return nil, err
	}
	if err = replace(l.extentTree, erFinal); err != nil {
		return nil, err
	}
	omapRecord := func(oid, addr uint64) record {
		k := append(u64(oid), u64(transaction)...)
		v := make([]byte, 16)
		put32(v, 4, blockSize)
		put64(v, 8, addr)
		return record{k, v}
	}
	for i, n := range l.fsTree.nodes {
		mapRecords[i] = omapRecord(n.oid, n.addr)
	}
	vm, err := buildTree(ctx, omapSpec, mapRecords)
	if err != nil {
		return nil, err
	}
	cm, err := buildTree(ctx, omapSpec, []record{omapRecord(volumeOID, l.volumeBlock)})
	if err != nil {
		return nil, err
	}
	if err = replace(l.volumeMapTree, vm); err != nil {
		return nil, err
	}
	if err = replace(l.containerMapTree, cm); err != nil {
		return nil, err
	}
	for _, t := range []*tree{l.fsTree, l.extentTree, l.snapTree, l.volumeMapTree, l.containerMapTree} {
		for _, n := range t.nodes {
			l.blocks[n.addr] = t.encode(n)
		}
	}
	l.blocks[l.containerMap] = encodeMap(l.containerMap, l.containerMapTree.root.oid, 1)
	l.blocks[l.volumeMap] = encodeMap(l.volumeMap, l.volumeMapTree.root.oid, 0)
	l.blocks[l.volumeBlock] = l.superblock()
	if err := l.spaceManager(ctx); err != nil {
		return nil, err
	}
	l.blocks[0] = l.container()
	l.blocks[2] = l.blocks[0]
	return l, nil
}

func encodeMap(addr, root uint64, flags uint32) []byte {
	b := make([]byte, blockSize)
	object(b, addr, physical|11, 0)
	put32(b, 32, flags)
	put32(b, 40, physical|2)
	put32(b, 44, physical|2)
	put64(b, 48, root)
	seal(b)
	return b
}

func (l *layout) uuid(kind string) []byte {
	h := sha256.New()
	_, _ = io.WriteString(h, "zapp APFS "+kind+"\x00"+l.volume.Name)
	_, _ = h.Write(u64(uint64(l.volume.Created.UnixNano())))
	if l.volume.CaseSensitive {
		_, _ = h.Write([]byte{1})
	}
	for _, e := range l.entries {
		_, _ = h.Write(u64(e.node.ID))
		_, _ = io.WriteString(h, e.node.Name+"\x00")
	}
	u := h.Sum(nil)[:16]
	u[6] = u[6]&15 | 0x50
	u[8] = u[8]&63 | 0x80
	return u
}

func (l *layout) container() []byte {
	b := make([]byte, blockSize)
	object(b, 1, ephemeral|1, 0)
	copy(b[32:], "NXSB")
	put32(b, 36, blockSize)
	put64(b, 40, l.total)
	put64(b, 64, 2)
	copy(b[72:], l.uuid("container"))
	put64(b, 88, l.nextOID)
	put64(b, 96, transaction+1)
	put32(b, 104, descBlocks)
	put32(b, 108, uint32(l.checkpointBlocks))
	put64(b, 112, 1)
	put64(b, 120, dataBase)
	put32(b, 128, 2)
	put32(b, 132, uint32(len(l.blocks[dataBase])/blockSize+3))
	put32(b, 140, 2)
	put32(b, 148, uint32(len(l.blocks[dataBase])/blockSize+3))
	put64(b, 152, spacemanOID)
	put64(b, 160, l.containerMap)
	put64(b, 168, reaperOID)
	put32(b, 180, 100)
	put64(b, 184, volumeOID)
	put64(b, 1312, 0x300040001) // Ephemeral format version, minimum blocks, maximum structures.
	seal(b)
	return b
}

func (l *layout) superblock() []byte {
	b := make([]byte, blockSize)
	object(b, volumeOID, 13, 0)
	copy(b[32:], "APSB")
	incompat := uint64(1)
	if l.volume.CaseSensitive {
		incompat = 8
	}
	put64(b, 56, incompat)
	put64(b, 88, l.volumeAllocated)
	put16(b, 96, 5)
	put32(b, 104, 6)
	put16(b, 112, 1) // Unencrypted metadata crypto state.
	put32(b, 116, 2)
	put32(b, 120, physical|2)
	put32(b, 124, physical|2)
	put64(b, 128, l.volumeMap)
	put64(b, 136, l.fsTree.root.oid)
	put64(b, 144, l.extentTree.root.oid)
	put64(b, 152, l.snapTree.root.oid)
	put64(b, 176, l.nextFileID)
	put64(b, 184, l.files)
	put64(b, 192, l.directories)
	put64(b, 200, l.symlinks)
	put64(b, 224, l.volumeAllocated)
	copy(b[240:], l.uuid("volume"))
	stamp := uint64(l.volume.Created.UnixNano())
	put64(b, 256, stamp)
	put64(b, 264, 1) // APFS_FS_UNENCRYPTED
	copy(b[272:], "zapp")
	put64(b, 304, stamp)
	put64(b, 312, transaction)
	copy(b[704:], l.volume.Name)
	put32(b, 960, 3)
	seal(b)
	return b
}

// write walks metadata and file extents in physical order. Empty space is
// streamed as zeroes; memory use does not depend on the sum of file sizes.
func (l *layout) write(ctx context.Context, w io.Writer) (int64, error) {
	type part struct {
		addr   uint64
		data   []byte
		stream *stream
	}
	parts := make([]part, 0, len(l.blocks)+len(l.streams))
	for addr, b := range l.blocks {
		parts = append(parts, part{addr: addr, data: b})
	}
	for _, s := range l.streams {
		parts = append(parts, part{addr: s.start, stream: s})
	}
	sort.Slice(parts, func(i, j int) bool {
		if parts[i].addr != parts[j].addr {
			return parts[i].addr < parts[j].addr
		}
		// Empty sources share the next extent's address but still need their
		// Open/size checks. Validate them before advancing past that address.
		return parts[i].stream != nil && parts[i].stream.size == 0 &&
			(parts[j].stream == nil || parts[j].stream.size != 0)
	})
	c := &imageWriter{ctx: ctx, w: w}
	for _, p := range parts {
		if err := c.zeros(int64(p.addr)*blockSize - c.n); err != nil {
			return c.n, err
		}
		if p.stream == nil {
			if _, err := c.Write(p.data); err != nil {
				return c.n, err
			}
			continue
		}
		s := p.stream
		r, err := s.source.Open()
		if err != nil {
			return c.n, err
		}
		written, err := io.CopyN(c, r, s.size)
		if err == nil {
			var extra [1]byte
			n, e := r.Read(extra[:])
			if n != 0 || e != io.EOF {
				err = fmt.Errorf("APFS source size changed after planning")
			}
		}
		closeErr := r.Close()
		if err != nil {
			return c.n, fmt.Errorf("APFS stream %d: copied %d of %d bytes: %w", s.id, written, s.size, err)
		}
		if closeErr != nil {
			return c.n, closeErr
		}
		if err := c.zeros(int64(s.blocks*blockSize) - s.size); err != nil {
			return c.n, err
		}
	}
	err := c.zeros(int64(l.total)*blockSize - c.n)
	return c.n, err
}

type imageWriter struct {
	ctx context.Context
	w   io.Writer
	n   int64
}

func (w *imageWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := w.w.Write(p)
	w.n += int64(n)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}
func (w *imageWriter) zeros(n int64) error {
	if n < 0 {
		return fmt.Errorf("APFS extents overlap by %d bytes", -n)
	}
	var b [32768]byte
	for n > 0 {
		count := min(n, int64(len(b)))
		if _, err := w.Write(b[:count]); err != nil {
			return err
		}
		n -= count
	}
	return w.ctx.Err()
}
