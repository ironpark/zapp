// Package udif writes UDIF disk images, the container a .dmg file uses. A raw
// disk image is split into chunks that are stored compressed, uncompressed, or
// not at all when they hold nothing but zeroes, and a table describing those
// chunks is appended along with the trailer that ties the file together.
// Reading images back is not supported.
package udif

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"

	"github.com/ironpark/zapp/pkg/lzfse"

	"github.com/ironpark/zapp/pkg/plist"
)

// SectorSize is the unit every offset and length in a UDIF image is counted in.
const SectorSize = 512

// chunkSectors is how much of the image one chunk covers. A megabyte is what
// Apple's own images use: large enough that the compressor has something to
// work with, small enough that mounting need not decompress much to read a
// little.
const chunkSectors = 2048

// Chunk types, as stored in the block table.
const (
	chunkZeroFill   = 0x00000000
	chunkRaw        = 0x00000001
	chunkZlib       = 0x80000005
	chunkLZFSE      = 0x80000007
	chunkTerminator = 0xFFFFFFFF
)

// Checksums throughout the format are CRC-32, identified by type 2 and a size
// given in bits.
const (
	checksumCRC32    = 2
	checksumBitCount = 32
)

// Format selects how the chunks of an image are compressed.
type Format int

const (
	// UDZO stores chunks with zlib. Every version of macOS reads it, which is
	// what makes it the safe choice for something being handed out.
	UDZO Format = iota
	// ULFO stores chunks with LZFSE, which is both smaller and quicker to read
	// back, but which only macOS 10.11 and later understand.
	ULFO
)

func (f Format) String() string {
	if f == ULFO {
		return "ULFO"
	}
	return "UDZO"
}

// Write compresses the raw disk image in src, which must be size bytes long,
// into a UDIF image in the given format. It returns the number of bytes
// written.
func Write(ctx context.Context, w io.Writer, src io.Reader, size int64, format Format) (int64, error) {
	return WriteWithOptions(ctx, w, src, size, Options{Format: format})
}

// DiskType identifies the filesystem in a whole-disk block table.
type DiskType string

const (
	AppleHFS  DiskType = "Apple_HFS"
	AppleAPFS DiskType = "Apple_APFS"
)

// Options selects compression and the whole-disk filesystem description.
// The zero value preserves Write's HFS+ / zlib behavior.
type Options struct {
	Format   Format
	DiskType DiskType
}

// WriteWithOptions is Write with an explicit filesystem description. It does
// not change the input image's partitioning or filesystem bytes.
func WriteWithOptions(ctx context.Context, w io.Writer, src io.Reader, size int64, options Options) (int64, error) {
	format := options.Format
	if options.DiskType == "" {
		options.DiskType = AppleHFS
	}
	if options.DiskType != AppleHFS && options.DiskType != AppleAPFS {
		return 0, fmt.Errorf("unknown disk type %q", options.DiskType)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if format != UDZO && format != ULFO {
		return 0, fmt.Errorf("unknown disk image format %d", format)
	}
	if size <= 0 {
		return 0, fmt.Errorf("image size must be positive")
	}
	if size%SectorSize != 0 {
		return 0, fmt.Errorf("image size %d is not a whole number of %d byte sectors", size, SectorSize)
	}

	table, dataLength, dataChecksum, err := writeChunks(ctx, w, src, size/SectorSize, format)
	if err != nil {
		return 0, err
	}

	xml, err := marshalTableForDisk(table, options.DiskType)
	if err != nil {
		return 0, err
	}
	if _, err := w.Write(xml); err != nil {
		return 0, err
	}

	trailer := encodeTrailer(trailerFields{
		dataForkLength: dataLength,
		dataChecksum:   dataChecksum,
		// The image checksum covers the checksums of the tables beneath it
		// rather than the data itself.
		masterChecksum: crc32.ChecksumIEEE(binary.BigEndian.AppendUint32(nil, table.checksum)),
		xmlOffset:      dataLength,
		xmlLength:      int64(len(xml)),
		sectorCount:    size / SectorSize,
	})
	if _, err := w.Write(trailer); err != nil {
		return 0, err
	}
	return dataLength + int64(len(xml)) + int64(len(trailer)), nil
}

// blockTable describes how the chunks written to the data fork map back onto
// the sectors of the original image.
type blockTable struct {
	sectorCount int64
	chunks      []chunk
	checksum    uint32 // CRC-32 of the uncompressed image.
}

type chunk struct {
	kind             uint32
	sector           int64
	sectors          int64
	compressedOffset int64
	compressedLength int64
}

// writeChunks streams the raw image out in compressed form, returning the table
// needed to find each chunk again.
func writeChunks(ctx context.Context, w io.Writer, src io.Reader, sectors int64, format Format) (blockTable, int64, uint32, error) {
	table := blockTable{sectorCount: sectors}
	sectorsPerChunk := int64(chunkSectors)
	if format == ULFO {
		// macOS's streaming LZFSE decoder can reject large final chunks even
		// when its buffer decoder verifies them. Keep streams below its
		// internal input window; this also bounds random-access decode work.
		sectorsPerChunk = 1024
	}
	raw := make([]byte, sectorsPerChunk*SectorSize)
	c := newCompressor(format)

	uncompressed := crc32.NewIEEE()
	data := crc32.NewIEEE()
	var offset int64

	for sector := int64(0); sector < sectors; {
		if err := ctx.Err(); err != nil {
			return table, 0, 0, err
		}
		count := min(sectorsPerChunk, sectors-sector)
		buf := raw[:count*SectorSize]
		if _, err := io.ReadFull(src, buf); err != nil {
			return table, 0, 0, fmt.Errorf("reading sector %d of %d: %w", sector, sectors, err)
		}
		_, _ = uncompressed.Write(buf)

		entry := chunk{sector: sector, sectors: count, compressedOffset: offset}
		switch payload := c.compress(buf); {
		case payload == nil:
			// A run of zeroes needs no storage at all: the reader fills it in.
			entry.kind = chunkZeroFill
		default:
			entry.kind = c.kind
			if len(payload) >= len(buf) {
				// Compression did not pay for itself, so store the sectors as
				// they are rather than make reading them cost more.
				entry.kind, payload = chunkRaw, buf
			}
			entry.compressedLength = int64(len(payload))
			if _, err := w.Write(payload); err != nil {
				return table, 0, 0, err
			}
			_, _ = data.Write(payload)
			offset += int64(len(payload))
		}
		table.chunks = append(table.chunks, entry)
		sector += count
	}

	// The table ends with a marker carrying no data of its own.
	table.chunks = append(table.chunks, chunk{kind: chunkTerminator, sector: sectors, compressedOffset: offset})
	table.checksum = uncompressed.Sum32()
	return table, offset, data.Sum32(), nil
}

// compressor holds whatever state one format needs between chunks, so that a
// codec with a large working set is set up once rather than per chunk.
type compressor struct {
	kind  uint32
	buf   bytes.Buffer
	lzfse *lzfse.Encoder
	out   []byte
}

func newCompressor(format Format) *compressor {
	if format == ULFO {
		return &compressor{kind: chunkLZFSE, lzfse: lzfse.NewEncoder()}
	}
	return &compressor{kind: chunkZlib}
}

// compress returns the stored form of one chunk, or nil when the chunk is
// entirely zeroes and so needs no storage at all.
func (c *compressor) compress(sectors []byte) []byte {
	zero := true
	for _, b := range sectors {
		if b != 0 {
			zero = false
			break
		}
	}
	if zero {
		return nil
	}
	if c.lzfse != nil {
		c.out = c.lzfse.Encode(c.out[:0], sectors)
		return c.out
	}
	c.buf.Reset()
	z := zlib.NewWriter(&c.buf)
	_, _ = z.Write(sectors)
	_ = z.Close()
	return c.buf.Bytes()
}

// mish is the signature of a block table, and the name UDIF gives the record
// holding one.
const mishSignature = 0x6D697368

// wholeDeviceDescriptor marks a table that covers a whole device rather than one
// partition of a partitioned one.
const wholeDeviceDescriptor = 0xFFFFFFFE

// encodeBlockTable writes the table as the format stores it: a fixed header
// describing the span of sectors it covers, followed by one 40 byte descriptor
// per chunk.
func encodeBlockTable(t blockTable) []byte {
	b := make([]byte, 204, 204+40*len(t.chunks))
	binary.BigEndian.PutUint32(b, mishSignature)
	binary.BigEndian.PutUint32(b[4:], 1) // Version.
	binary.BigEndian.PutUint64(b[8:], 0) // First sector covered.
	binary.BigEndian.PutUint64(b[16:], uint64(t.sectorCount))
	binary.BigEndian.PutUint64(b[24:], 0)     // Where this table's data starts.
	binary.BigEndian.PutUint32(b[32:], 0x808) // Buffers a reader should keep ready.
	binary.BigEndian.PutUint32(b[36:], wholeDeviceDescriptor)
	binary.BigEndian.PutUint32(b[64:], checksumCRC32)
	binary.BigEndian.PutUint32(b[68:], checksumBitCount)
	binary.BigEndian.PutUint32(b[72:], t.checksum)
	binary.BigEndian.PutUint32(b[200:], uint32(len(t.chunks)))

	for _, c := range t.chunks {
		entry := make([]byte, 40)
		binary.BigEndian.PutUint32(entry, c.kind)
		binary.BigEndian.PutUint64(entry[8:], uint64(c.sector))
		binary.BigEndian.PutUint64(entry[16:], uint64(c.sectors))
		binary.BigEndian.PutUint64(entry[24:], uint64(c.compressedOffset))
		binary.BigEndian.PutUint64(entry[32:], uint64(c.compressedLength))
		b = append(b, entry...)
	}
	return b
}

// marshalTable wraps the block table in the property list the trailer points
// at. The format inherits its shape from a classic resource fork, so the table
// appears as a single numbered resource.
func marshalTable(t blockTable) ([]byte, error) {
	return marshalTableForDisk(t, AppleHFS)
}

func marshalTableForDisk(t blockTable, diskType DiskType) ([]byte, error) {
	name := fmt.Sprintf("whole disk (%s : 0)", diskType)
	return plist.MarshalXML(map[string]any{
		"resource-fork": map[string]any{
			"blkx": []any{map[string]any{
				"Attributes": "0x0050",
				"CFName":     name,
				"Name":       name,
				"ID":         "0",
				"Data":       encodeBlockTable(t),
			}},
		},
	})
}

// trailerSize is the fixed length of the koly trailer that ends every UDIF
// image, and the thing a reader looks for first.
const trailerSize = 512

const kolySignature = 0x6B6F6C79

type trailerFields struct {
	dataForkLength int64
	dataChecksum   uint32
	masterChecksum uint32
	xmlOffset      int64
	xmlLength      int64
	sectorCount    int64
}

// encodeTrailer writes the koly trailer. A reader finds it by seeking to the
// last 512 bytes of the file, which is why everything else can be streamed.
func encodeTrailer(f trailerFields) []byte {
	b := make([]byte, trailerSize)
	binary.BigEndian.PutUint32(b, kolySignature)
	binary.BigEndian.PutUint32(b[4:], 4)           // Version.
	binary.BigEndian.PutUint32(b[8:], trailerSize) // Size of this trailer.
	binary.BigEndian.PutUint32(b[12:], 1)          // Flags.
	binary.BigEndian.PutUint64(b[32:], uint64(f.dataForkLength))
	binary.BigEndian.PutUint32(b[56:], 1) // This segment,
	binary.BigEndian.PutUint32(b[60:], 1) // of one: the image is not split.

	binary.BigEndian.PutUint32(b[80:], checksumCRC32)
	binary.BigEndian.PutUint32(b[84:], checksumBitCount)
	binary.BigEndian.PutUint32(b[88:], f.dataChecksum)

	binary.BigEndian.PutUint64(b[216:], uint64(f.xmlOffset))
	binary.BigEndian.PutUint64(b[224:], uint64(f.xmlLength))

	binary.BigEndian.PutUint32(b[352:], checksumCRC32)
	binary.BigEndian.PutUint32(b[356:], checksumBitCount)
	binary.BigEndian.PutUint32(b[360:], f.masterChecksum)

	// Image variant 2 is a whole device; 1 describes a single partition, which
	// a reader then expects to find a partition map for.
	binary.BigEndian.PutUint32(b[488:], 2)
	binary.BigEndian.PutUint64(b[492:], uint64(f.sectorCount))
	return b
}
