package lzfse

import (
	"bytes"
	"math/rand"
	"os"
	"testing"
)

// benchCorpus is a mix of runs, text and noise, which exercises both long
// matches and stretches with none.
func benchCorpus(n int) []byte {
	r := rand.New(rand.NewSource(7))
	var b []byte
	for len(b) < n {
		switch r.Intn(3) {
		case 0:
			b = append(b, bytes.Repeat([]byte{byte(r.Intn(256))}, 1+r.Intn(4000))...)
		case 1:
			b = append(b, bytes.Repeat([]byte("the quick brown fox jumps over "), 1+r.Intn(50))...)
		default:
			chunk := make([]byte, 1+r.Intn(2000))
			r.Read(chunk)
			b = append(b, chunk...)
		}
	}
	return b[:n]
}

func BenchmarkEncode(b *testing.B) {
	src := benchCorpus(8 << 20)
	e := NewEncoder()
	dst := make([]byte, 0, len(src))
	b.SetBytes(int64(len(src)))
	b.ResetTimer()
	for b.Loop() {
		dst = e.Encode(dst[:0], src)
	}
}

func BenchmarkEncodeBinary(b *testing.B) {
	src, err := os.ReadFile(os.Getenv("LZFSE_BENCH_FILE"))
	if err != nil {
		b.Skip("set LZFSE_BENCH_FILE to a real file")
	}
	e := NewEncoder()
	dst := make([]byte, 0, len(src))
	b.SetBytes(int64(len(src)))
	b.ResetTimer()
	for b.Loop() {
		dst = e.Encode(dst[:0], src)
	}
}

// BenchmarkEncodeChunks mirrors how a disk image is compressed: many separate
// streams of a megabyte each, through one reused Encoder.
func BenchmarkEncodeChunks(b *testing.B) {
	const chunk = 1 << 20
	src := benchCorpus(16 * chunk)
	e := NewEncoder()
	dst := make([]byte, 0, chunk)
	b.SetBytes(int64(len(src)))
	b.ResetTimer()
	for b.Loop() {
		for off := 0; off < len(src); off += chunk {
			dst = e.Encode(dst[:0], src[off:off+chunk])
		}
	}
}
