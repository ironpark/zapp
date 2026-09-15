package hfsplus

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"time"
)

// Volume header constants. 'H+' with version 4 is an unjournaled HFS+ volume;
// the unmounted bit is what tells the driver the volume was closed cleanly and
// so needs no repair before it can be mounted.
const (
	volumeSignature   = 0x482B
	volumeVersion     = 4
	volumeUnmounted   = 0x00000100
	lastMountedZapp   = 0x7A617070 // 'zapp'
	volumeHeaderStart = 1024
	volumeHeaderSize  = 512
	sectorSize        = 512

	defaultBlockSize = 4096
	catalogNodeSize  = 8192
	extentsNodeSize  = 4096
)

// File modes as HFS+ records them, which are the POSIX values.
const (
	modeDir     = 0o040000
	modeRegular = 0o100000
	modeSymlink = 0o120000
)

// fork is one fork's placement once the layout is decided.
type fork struct {
	logicalSize int64
	startBlock  uint32
	blockCount  uint32
	source      Source
}

func (f fork) encode() []byte {
	b := make([]byte, forkDataSize)
	binary.BigEndian.PutUint64(b, uint64(f.logicalSize))
	binary.BigEndian.PutUint32(b[12:], f.blockCount)
	binary.BigEndian.PutUint32(b[16:], f.startBlock)
	binary.BigEndian.PutUint32(b[20:], f.blockCount)
	return b
}

// Write lays out and writes the whole image, returning its size in bytes. The
// tree is numbered first if it has not been numbered already.
func Write(ctx context.Context, w io.Writer, v Volume) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if v.Name == "" {
		return 0, fmt.Errorf("volume name is required")
	}
	if _, err := encodeName(v.Name); err != nil {
		return 0, fmt.Errorf("volume name: %w", err)
	}
	if v.BlockSize == 0 {
		v.BlockSize = defaultBlockSize
	}
	if v.BlockSize < defaultBlockSize || v.BlockSize&(v.BlockSize-1) != 0 {
		return 0, fmt.Errorf("block size must be a power of two of at least %d", defaultBlockSize)
	}
	if v.FreeSpace < 0 {
		return 0, fmt.Errorf("free space cannot be negative")
	}
	if v.Root == nil || v.Root.ID == 0 {
		if err := AssignIDs(&v); err != nil {
			return 0, err
		}
	}
	if v.Created.IsZero() {
		v.Created = time.Now()
	}

	plan, err := planVolume(ctx, v)
	if err != nil {
		return 0, err
	}
	return plan.write(ctx, w)
}

// layout is a fully decided image: every fork has a home and every structure
// has a size, so writing is a straight walk from the first byte to the last.
type layout struct {
	volume      Volume
	blockSize   uint32
	totalBlocks uint32

	bitmap  fork
	extents fork
	catalog fork
	forks   []fork // Data and resource forks, in the order they are written.

	catalogData []byte
	extentsData []byte

	fileCount   uint32
	folderCount uint32
	usedBlocks  uint32
}

// blocksFor is the number of allocation blocks n bytes occupy.
func blocksFor(n int64, blockSize uint32) uint32 {
	return uint32((n + int64(blockSize) - 1) / int64(blockSize))
}

// item is one catalog entry paired with the forks it owns. Forks are referred
// to by index because the slice holding them is still growing while the tree is
// being walked.
type item struct {
	node     *Node
	parentID uint32
	dataFork int // Index into layout.forks, or -1.
	rsrcFork int
}

// collect walks the tree in catalog order, recording every entry and reserving
// a slot for each fork that will need somewhere to live.
func (l *layout) collect(dir *Node) ([]item, error) {
	var items []item
	var walk func(*Node) error
	walk = func(parent *Node) error {
		for _, n := range parent.Children {
			it := item{node: n, parentID: parent.ID, dataFork: -1, rsrcFork: -1}
			switch {
			case n.IsDir():
				l.folderCount++
			case n.IsSymlink():
				if n.LinkTarget == "" {
					return fmt.Errorf("%s: symbolic link has no target", n.Name)
				}
				l.fileCount++
				it.dataFork = len(l.forks)
				l.forks = append(l.forks, fork{logicalSize: int64(len(n.LinkTarget)), source: Bytes([]byte(n.LinkTarget))})
			default:
				l.fileCount++
				if n.Data != nil {
					it.dataFork = len(l.forks)
					l.forks = append(l.forks, fork{logicalSize: n.Data.Size(), source: n.Data})
				}
				if n.ResourceFork != nil {
					it.rsrcFork = len(l.forks)
					l.forks = append(l.forks, fork{logicalSize: n.ResourceFork.Size(), source: n.ResourceFork})
				}
			}
			items = append(items, it)
		}
		for _, n := range parent.Children {
			if n.IsDir() {
				if err := walk(n); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(dir); err != nil {
		return nil, err
	}
	return items, nil
}

// buildCatalog encodes the catalog B-tree. Record sizes do not depend on where
// the forks ended up, so calling this before the layout is decided gives a tree
// of exactly the size the final one will have.
func (l *layout) buildCatalog(ctx context.Context, items []item) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rootName, err := encodeName(l.volume.Name)
	if err != nil {
		return nil, err
	}

	type entry struct {
		key  catalogKey
		data []byte
	}
	entries := make([]entry, 0, 2*len(items)+2)
	add := func(key catalogKey, data []byte) {
		entries = append(entries, entry{key, data})
	}

	// The root folder is keyed under the reserved parent ID and named after the
	// volume, and its thread record names it the same way.
	add(catalogKey{rootParentID, rootName}, l.encodeFolder(l.volume.Root, rootFolderID))
	add(catalogKey{rootFolderID, nil}, encodeThread(recordFolderThread, rootParentID, rootName))

	for _, it := range items {
		name, err := encodeName(it.node.Name)
		if err != nil {
			return nil, err
		}
		if it.node.IsDir() {
			add(catalogKey{it.parentID, name}, l.encodeFolder(it.node, it.node.ID))
			add(catalogKey{it.node.ID, nil}, encodeThread(recordFolderThread, it.parentID, name))
			continue
		}
		add(catalogKey{it.parentID, name}, l.encodeFile(it))
		add(catalogKey{it.node.ID, nil}, encodeThread(recordFileThread, it.parentID, name))
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].key.compare(entries[j].key) < 0 })
	records := make([]btreeRecord, len(entries))
	for i, e := range entries {
		records[i] = btreeRecord{key: e.key.encode(), data: e.data}
	}
	return buildBTree(btreeSpec{
		nodeSize:       catalogNodeSize,
		maxKeyLength:   maxCatalogKeyLen,
		keyCompareType: compareCaseFolding,
		attributes:     attrBigKeys | attrVariableIndexKey,
	}, records)
}

// encodeFolder writes a catalog folder record.
func (l *layout) encodeFolder(n *Node, id uint32) []byte {
	b := make([]byte, folderRecordSize)
	binary.BigEndian.PutUint16(b, recordFolder)
	// A folder always owns a thread record, so unlike a file it carries no flag
	// saying so and its flags field stays zero.
	binary.BigEndian.PutUint32(b[4:], uint32(len(n.Children)))
	binary.BigEndian.PutUint32(b[8:], id)
	l.putTimes(b[12:], n.ModTime)
	putPermissions(b[32:], modeDir|0o755)
	copy(b[48:], n.FinderInfo[:])
	return b
}

// encodeFile writes a catalog file record, including the placement of its forks.
func (l *layout) encodeFile(it item) []byte {
	n := it.node
	b := make([]byte, fileRecordSize)
	binary.BigEndian.PutUint16(b, recordFile)
	binary.BigEndian.PutUint16(b[2:], flagThreadExists)
	binary.BigEndian.PutUint32(b[8:], n.ID)
	l.putTimes(b[12:], n.ModTime)

	mode := uint16(modeRegular | 0o644)
	finder := n.FinderInfo
	if n.IsSymlink() {
		// The Finder recognises a symbolic link by its type and creator, and
		// the driver by the file mode; both have to agree.
		mode = modeSymlink | 0o755
		copy(finder[0:], "slnk")
		copy(finder[4:], "rhap")
	} else if perm := n.Mode.Perm(); perm != 0 {
		mode = modeRegular | uint16(perm)
	}
	putPermissions(b[32:], mode)
	copy(b[48:], finder[:])

	if it.dataFork >= 0 {
		copy(b[88:], l.forks[it.dataFork].encode())
	}
	if it.rsrcFork >= 0 {
		copy(b[168:], l.forks[it.rsrcFork].encode())
	}
	return b
}

// putTimes writes the five timestamps every catalog record carries. HFS+ keeps
// the creation date in local time and the rest in GMT, but a freshly built
// image has no history, so all of them describe the one moment.
func (l *layout) putTimes(b []byte, modTime time.Time) {
	if modTime.IsZero() {
		modTime = l.volume.Created
	}
	created := hfsTime(l.volume.Created)
	stamp := hfsTime(modTime)
	binary.BigEndian.PutUint32(b, created)
	binary.BigEndian.PutUint32(b[4:], stamp)
	binary.BigEndian.PutUint32(b[8:], stamp)
	binary.BigEndian.PutUint32(b[12:], stamp)
}

// putPermissions writes a HFSPlusBSDInfo owned by root and wheel.
func putPermissions(b []byte, mode uint16) {
	binary.BigEndian.PutUint16(b[10:], mode)
}

// encodeThread writes the record that maps a catalog node ID back to its name
// and parent, which every folder and file in the catalog has.
func encodeThread(recordType uint16, parentID uint32, name []uint16) []byte {
	b := make([]byte, 10+2*len(name))
	binary.BigEndian.PutUint16(b, recordType)
	binary.BigEndian.PutUint32(b[4:], parentID)
	binary.BigEndian.PutUint16(b[8:], uint16(len(name)))
	for i, u := range name {
		binary.BigEndian.PutUint16(b[10+2*i:], u)
	}
	return b
}

// planVolume decides the size and position of everything in the image. The
// catalog is built twice: its record sizes do not depend on where the forks
// land, so the first pass establishes how much room the tree needs, and once
// that fixes where the file data begins, the second pass records the extents
// the data actually landed on.
func planVolume(ctx context.Context, v Volume) (*layout, error) {
	l := &layout{volume: v, blockSize: v.BlockSize}

	// An empty extents overflow tree, which every volume must have even when
	// nothing overflows into it. Every fork here is contiguous, so none does.
	extentsData, err := buildBTree(btreeSpec{
		nodeSize: extentsNodeSize, maxKeyLength: 10, attributes: attrBigKeys,
	}, nil)
	if err != nil {
		return nil, err
	}
	l.extentsData = extentsData

	items, err := l.collect(v.Root)
	if err != nil {
		return nil, err
	}
	catalogData, err := l.buildCatalog(ctx, items)
	if err != nil {
		return nil, err
	}

	// Bytes 0 to 1535 hold the boot blocks and the volume header; the last 1024
	// hold the alternate header and a trailing reserved sector.
	reservedBlocks := blocksFor(volumeHeaderStart+volumeHeaderSize, l.blockSize)
	tailBlocks := blocksFor(2*sectorSize, l.blockSize)
	extentsBlocks := blocksFor(int64(len(extentsData)), l.blockSize)
	catalogBlocks := blocksFor(int64(len(catalogData)), l.blockSize)
	freeBlocks := blocksFor(v.FreeSpace, l.blockSize)

	var dataBlocks uint32
	for i := range l.forks {
		if l.forks[i].logicalSize < 0 {
			return nil, fmt.Errorf("fork reports a negative size")
		}
		dataBlocks += blocksFor(l.forks[i].logicalSize, l.blockSize)
	}

	// The bitmap describes the volume it is itself part of, so grow it until it
	// is large enough to cover the volume its own size produces.
	bitmapBlocks := uint32(1)
	for {
		total := reservedBlocks + bitmapBlocks + extentsBlocks + catalogBlocks + dataBlocks + freeBlocks + tailBlocks
		needed := blocksFor(int64((total+7)/8), l.blockSize)
		if needed <= bitmapBlocks {
			l.totalBlocks = total
			break
		}
		bitmapBlocks = needed
	}

	next := reservedBlocks
	place := func(size int64, blocks uint32) fork {
		f := fork{logicalSize: size, startBlock: next, blockCount: blocks}
		next += blocks
		return f
	}
	l.bitmap = place(int64((l.totalBlocks+7)/8), bitmapBlocks)
	l.extents = place(int64(len(extentsData)), extentsBlocks)
	l.catalog = place(int64(len(catalogData)), catalogBlocks)
	for i := range l.forks {
		blocks := blocksFor(l.forks[i].logicalSize, l.blockSize)
		l.forks[i].startBlock = next
		l.forks[i].blockCount = blocks
		next += blocks
	}
	l.usedBlocks = next

	// The same records, now carrying the extents just assigned.
	l.catalogData, err = l.buildCatalog(ctx, items)
	if err != nil {
		return nil, err
	}
	if len(l.catalogData) != len(catalogData) {
		return nil, fmt.Errorf("catalog changed size between layout passes")
	}
	return l, nil
}

// encodeHeader writes the volume header, which the image carries twice.
func (l *layout) encodeHeader() []byte {
	b := make([]byte, volumeHeaderSize)
	binary.BigEndian.PutUint16(b, volumeSignature)
	binary.BigEndian.PutUint16(b[2:], volumeVersion)
	binary.BigEndian.PutUint32(b[4:], volumeUnmounted)
	binary.BigEndian.PutUint32(b[8:], lastMountedZapp)

	stamp := hfsTime(l.volume.Created)
	binary.BigEndian.PutUint32(b[16:], stamp) // Created, in local time by definition.
	binary.BigEndian.PutUint32(b[20:], stamp)
	binary.BigEndian.PutUint32(b[28:], stamp) // Last checked, so fsck is not implied.

	binary.BigEndian.PutUint32(b[32:], l.fileCount)
	binary.BigEndian.PutUint32(b[36:], l.folderCount)
	binary.BigEndian.PutUint32(b[40:], l.blockSize)
	binary.BigEndian.PutUint32(b[44:], l.totalBlocks)
	binary.BigEndian.PutUint32(b[48:], l.totalBlocks-l.usedBlocks-blocksFor(2*sectorSize, l.blockSize))
	binary.BigEndian.PutUint32(b[52:], l.usedBlocks)
	binary.BigEndian.PutUint32(b[56:], l.blockSize) // Resource clump size.
	binary.BigEndian.PutUint32(b[60:], l.blockSize) // Data clump size.
	binary.BigEndian.PutUint32(b[64:], firstUserID+l.fileCount+l.folderCount)
	binary.BigEndian.PutUint32(b[68:], 1) // Write count.
	binary.BigEndian.PutUint64(b[72:], 1) // Only the MacRoman encoding is in use.

	copy(b[112:], l.bitmap.encode())
	copy(b[192:], l.extents.encode())
	copy(b[272:], l.catalog.encode())
	// The attributes and startup files stay absent, and so all zero.
	return b
}

// encodeBitmap marks every block the image occupies as in use. Allocation is
// contiguous from the front, apart from the reserved blocks at the very end.
func (l *layout) encodeBitmap() []byte {
	b := make([]byte, int64(l.bitmap.blockCount)*int64(l.blockSize))
	set := func(from, to uint32) {
		for i := from; i < to; i++ {
			b[i/8] |= 0x80 >> (i % 8)
		}
	}
	set(0, l.usedBlocks)
	set(l.totalBlocks-blocksFor(2*sectorSize, l.blockSize), l.totalBlocks)
	return b
}

// write emits the image from the first byte to the last. Everything the layout
// decided sits in ascending order, so a single forward pass produces the whole
// volume without ever seeking.
func (l *layout) write(ctx context.Context, w io.Writer) (int64, error) {
	c := &counter{w: w, ctx: ctx}
	header := l.encodeHeader()

	// The boot blocks, then the volume header, then the rest of its block.
	c.zeros(volumeHeaderStart)
	c.write(header)
	c.pad(l.blockSize)

	c.write(l.encodeBitmap())
	c.write(l.extentsData)
	c.pad(l.blockSize)
	c.write(l.catalogData)
	c.pad(l.blockSize)

	for i := range l.forks {
		if err := c.copyFork(l.forks[i]); err != nil {
			return c.n, err
		}
		c.pad(l.blockSize)
	}

	// Free space, then the alternate header in the final reserved blocks.
	total := int64(l.totalBlocks) * int64(l.blockSize)
	c.zeros(total - 2*sectorSize - c.n)
	c.write(header)
	c.zeros(sectorSize)

	if c.err != nil {
		return c.n, c.err
	}
	if c.n != total {
		return c.n, fmt.Errorf("wrote %d bytes for a volume planned at %d", c.n, total)
	}
	return c.n, nil
}

// copyFork streams one fork's contents, checking that the source is the size it
// claimed when the layout was decided.
func (c *counter) copyFork(f fork) error {
	if c.err != nil || f.source == nil {
		return c.err
	}
	r, err := f.source.Open()
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	n, err := io.Copy(c, r)
	if err != nil {
		return err
	}
	if n != f.logicalSize {
		return fmt.Errorf("fork reported %d bytes but produced %d", f.logicalSize, n)
	}
	return nil
}

// counter writes forward, tracking the offset so that padding and the position
// of the alternate header can be worked out as it goes, and holding on to the
// first error so each step does not have to be checked in turn.
type counter struct {
	w   io.Writer
	ctx context.Context
	n   int64
	err error
}

func (c *counter) Write(p []byte) (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	if err := c.ctx.Err(); err != nil {
		c.err = err
		return 0, err
	}
	n, err := c.w.Write(p)
	c.n += int64(n)
	if err != nil {
		c.err = err
	}
	return n, err
}

func (c *counter) write(p []byte) {
	_, _ = c.Write(p)
}

// zeros writes n zero bytes, in chunks so that a large gap costs no memory.
func (c *counter) zeros(n int64) {
	if n < 0 {
		c.err = fmt.Errorf("image layout overran by %d bytes", -n)
		return
	}
	var chunk [32 * 1024]byte
	for n > 0 && c.err == nil {
		size := int64(len(chunk))
		if n < size {
			size = n
		}
		c.write(chunk[:size])
		n -= size
	}
}

// pad advances to the next allocation block boundary.
func (c *counter) pad(blockSize uint32) {
	if remainder := c.n % int64(blockSize); remainder != 0 {
		c.zeros(int64(blockSize) - remainder)
	}
}
