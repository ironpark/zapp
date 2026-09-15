package udif

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ironpark/zapp/pkg/plist"
)

// parsed is what a reader recovers from an image: the trailer's view of the
// file and the chunk table describing how to put the sectors back together.
type parsed struct {
	dataForkLength int64
	sectorCount    int64
	chunks         []chunk
}

// parseImage reads an image back the way a reader would, starting from the
// trailer at the very end and following it to the chunk table.
func parseImage(t *testing.T, image []byte) parsed {
	t.Helper()
	if len(image) < trailerSize {
		t.Fatal("image is shorter than its trailer")
	}
	trailer := image[len(image)-trailerSize:]
	if got := binary.BigEndian.Uint32(trailer); got != kolySignature {
		t.Fatalf("trailer signature is %#x", got)
	}
	out := parsed{
		dataForkLength: int64(binary.BigEndian.Uint64(trailer[32:])),
		sectorCount:    int64(binary.BigEndian.Uint64(trailer[492:])),
	}
	xmlOffset := binary.BigEndian.Uint64(trailer[216:])
	xmlLength := binary.BigEndian.Uint64(trailer[224:])
	if int(xmlOffset+xmlLength) != len(image)-trailerSize {
		t.Fatalf("the table does not sit between the data and the trailer")
	}

	dict, err := plist.ParseDict(image[xmlOffset : xmlOffset+xmlLength])
	if err != nil {
		t.Fatal(err)
	}
	entries := dict["resource-fork"].(map[string]any)["blkx"].([]any)
	if len(entries) != 1 {
		t.Fatalf("got %d block tables, want 1", len(entries))
	}
	table := entries[0].(map[string]any)["Data"].([]byte)
	if got := binary.BigEndian.Uint32(table); got != mishSignature {
		t.Fatalf("block table signature is %#x", got)
	}
	count := int(binary.BigEndian.Uint32(table[200:]))
	for i := range count {
		e := table[204+40*i:]
		out.chunks = append(out.chunks, chunk{
			kind:             binary.BigEndian.Uint32(e),
			sector:           int64(binary.BigEndian.Uint64(e[8:])),
			sectors:          int64(binary.BigEndian.Uint64(e[16:])),
			compressedOffset: int64(binary.BigEndian.Uint64(e[24:])),
			compressedLength: int64(binary.BigEndian.Uint64(e[32:])),
		})
	}
	return out
}

// build compresses raw into an image and parses the result.
func build(t *testing.T, raw []byte) ([]byte, parsed) {
	t.Helper()
	var out bytes.Buffer
	n, err := Write(context.Background(), &out, bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(out.Len()) {
		t.Fatalf("reported %d bytes but wrote %d", n, out.Len())
	}
	return out.Bytes(), parseImage(t, out.Bytes())
}

// checkChunks confirms the table covers every sector exactly once, in order,
// and that the stored data sits where each chunk says it does.
func checkChunks(t *testing.T, p parsed) {
	t.Helper()
	var sector, offset int64
	for i, c := range p.chunks {
		if c.sector != sector {
			t.Fatalf("chunk %d starts at sector %d, want %d", i, c.sector, sector)
		}
		if c.compressedOffset != offset {
			t.Fatalf("chunk %d reads from %d, want %d", i, c.compressedOffset, offset)
		}
		if c.kind == chunkTerminator {
			if i != len(p.chunks)-1 {
				t.Fatalf("the table ends at chunk %d of %d", i, len(p.chunks))
			}
			return
		}
		sector += c.sectors
		offset += c.compressedLength
	}
	t.Fatal("the table has no terminator")
}

func TestZeroRunsAreNotStored(t *testing.T) {
	raw := make([]byte, 8*chunkSectors*SectorSize)
	image, p := build(t, raw)
	checkChunks(t, p)

	if p.sectorCount != int64(len(raw))/SectorSize {
		t.Fatalf("image covers %d sectors, want %d", p.sectorCount, int64(len(raw))/SectorSize)
	}
	if p.dataForkLength != 0 {
		t.Fatalf("an empty image stored %d bytes of data", p.dataForkLength)
	}
	for i, c := range p.chunks[:len(p.chunks)-1] {
		if c.kind != chunkZeroFill {
			t.Fatalf("chunk %d has type %#x, want a run of zeroes", i, c.kind)
		}
	}
	// Nothing but the table and the trailer should be left.
	if len(image) > 4096 {
		t.Fatalf("an image of nothing came to %d bytes", len(image))
	}
}

func TestIncompressibleDataIsStoredRaw(t *testing.T) {
	raw := make([]byte, 2*chunkSectors*SectorSize)
	rand.New(rand.NewSource(1)).Read(raw)
	_, p := build(t, raw)
	checkChunks(t, p)

	for i, c := range p.chunks[:len(p.chunks)-1] {
		if c.kind != chunkRaw {
			t.Fatalf("chunk %d has type %#x, want raw storage", i, c.kind)
		}
		if c.compressedLength != c.sectors*SectorSize {
			t.Fatalf("chunk %d stored %d bytes for %d sectors", i, c.compressedLength, c.sectors)
		}
	}
	// Storing random data raw must not make the image larger than the original.
	if p.dataForkLength != int64(len(raw)) {
		t.Fatalf("stored %d bytes for a %d byte image", p.dataForkLength, len(raw))
	}
}

func TestCompressibleDataShrinks(t *testing.T) {
	raw := bytes.Repeat([]byte("zapp builds installers. "), 3*chunkSectors*SectorSize/24)
	raw = raw[:3*chunkSectors*SectorSize]
	_, p := build(t, raw)
	checkChunks(t, p)

	for i, c := range p.chunks[:len(p.chunks)-1] {
		if c.kind != chunkZlib {
			t.Fatalf("chunk %d has type %#x, want compressed storage", i, c.kind)
		}
	}
	if p.dataForkLength > int64(len(raw))/10 {
		t.Fatalf("repetitive data only shrank to %d of %d bytes", p.dataForkLength, len(raw))
	}
}

func TestPartialFinalChunk(t *testing.T) {
	// A size that is a whole number of sectors but not of chunks.
	raw := make([]byte, chunkSectors*SectorSize+3*SectorSize)
	rand.New(rand.NewSource(2)).Read(raw)
	_, p := build(t, raw)
	checkChunks(t, p)

	last := p.chunks[len(p.chunks)-2]
	if last.sectors != 3 {
		t.Fatalf("the final chunk covers %d sectors, want 3", last.sectors)
	}
}

func TestRejectsBadSize(t *testing.T) {
	for name, size := range map[string]int64{
		"zero":          0,
		"negative":      -512,
		"part sector":   1000,
		"short of size": 4 * SectorSize,
	} {
		t.Run(name, func(t *testing.T) {
			src := bytes.NewReader(make([]byte, 512))
			if _, err := Write(context.Background(), new(bytes.Buffer), src, size); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestHonoursContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src := bytes.NewReader(make([]byte, SectorSize))
	if _, err := Write(ctx, new(bytes.Buffer), src, SectorSize); err != context.Canceled {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

// TestSystemAcceptsImage checks the container against the system's own reader,
// which validates the trailer, the table and both checksums. The payload need
// not be a filesystem for that.
func TestSystemAcceptsImage(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("checking an image needs macOS")
	}
	// A mix of all three chunk kinds, so the reader has to handle each of them
	// and reports the image as compressed.
	raw := make([]byte, 3*chunkSectors*SectorSize)
	copy(raw, bytes.Repeat([]byte("zapp "), chunkSectors*SectorSize/5))
	rand.New(rand.NewSource(3)).Read(raw[chunkSectors*SectorSize : 2*chunkSectors*SectorSize])
	image, _ := build(t, raw)

	path := filepath.Join(t.TempDir(), "image.dmg")
	if err := os.WriteFile(path, image, 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("hdiutil", "verify", path).CombinedOutput(); err != nil {
		t.Fatalf("verify rejected the image: %v\n%s", err, out)
	}
	out, err := exec.Command("hdiutil", "imageinfo", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "\nFormat: UDZO") {
		t.Fatalf("the image was not read as UDZO:\n%s", out)
	}
}
