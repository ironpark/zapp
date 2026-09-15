package udif

import (
	"bytes"
	"math/rand"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// DiskImages uses the streaming decoder. Buffer decoding and hdiutil verify
// alone miss failures with large, partially compressible final chunks.
func TestLZFSEStreamingCompatibility(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("needs the macOS Compression framework")
	}
	decoder := filepath.Join(t.TempDir(), "decode")
	if out, err := exec.Command("clang", "testdata/decode_lzfse.c", "-lcompression", "-o", decoder).CombinedOutput(); err != nil {
		t.Fatalf("compile decoder: %v\n%s", err, out)
	}
	for _, sectors := range []int{1984, 2048, 4100} {
		raw := make([]byte, sectors*SectorSize)
		rand.New(rand.NewSource(11)).Read(raw)
		// Leave enough repetition to select LZFSE while retaining a large
		// compressed payload, including in the incomplete final chunk.
		for i := 0; i < len(raw); i += 4096 {
			clear(raw[i : i+512])
		}
		image, parsed := build(t, raw, ULFO)
		checkChunks(t, parsed)
		for _, c := range parsed.chunks {
			if c.kind != chunkLZFSE {
				continue
			}
			cmd := exec.Command(decoder)
			cmd.Stdin = bytes.NewReader(image[c.compressedOffset : c.compressedOffset+c.compressedLength])
			var out, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("%d sectors, chunk at %d: %v\n%s", sectors, c.sector, err, stderr.String())
			}
			if !bytes.Equal(out.Bytes(), raw[c.sector*SectorSize:(c.sector+c.sectors)*SectorSize]) {
				t.Fatal("stream decoder returned different bytes")
			}
		}
	}
}
