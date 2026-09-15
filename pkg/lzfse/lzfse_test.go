package lzfse

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"os"
	"os/exec"
	"testing"
)

// corpus returns data of a shape named by kind, which is how the tests cover
// long matches, stretches with none, and everything in between.
func corpus(kind string, n int, seed int64) []byte {
	r := rand.New(rand.NewSource(seed))
	switch kind {
	case "zeros":
		return make([]byte, n)
	case "random":
		b := make([]byte, n)
		r.Read(b)
		return b
	case "repeat":
		return bytes.Repeat([]byte("abcdefgh"), n/8+1)[:n]
	case "text":
		words := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
		var b []byte
		for len(b) < n {
			b = append(b, words[r.Intn(len(words))]...)
			b = append(b, ' ')
		}
		return b[:n]
	case "runs":
		var b []byte
		for len(b) < n {
			b = append(b, bytes.Repeat([]byte{byte(r.Intn(256))}, 1+r.Intn(400))...)
		}
		return b[:n]
	case "mixed":
		var b []byte
		for len(b) < n {
			if r.Intn(2) == 0 {
				chunk := make([]byte, 1+r.Intn(600))
				r.Read(chunk)
				b = append(b, chunk...)
			} else {
				b = append(b, bytes.Repeat([]byte("the quick brown fox "), 1+r.Intn(40))...)
			}
		}
		return b[:n]
	}
	panic("unknown corpus " + kind)
}

var kinds = []string{"zeros", "random", "repeat", "text", "runs", "mixed"}

var sizes = []int{0, 1, 7, 8, 9, 16, 255, 256, 1000, 4096, 65536, 300000}

// TestStreamFraming checks that whatever the input, the result is a sequence of
// blocks that ends where a decoder will look for the end.
func TestStreamFraming(t *testing.T) {
	for _, kind := range kinds {
		for _, n := range sizes {
			out := Encode(nil, corpus(kind, n, 1))
			if len(out) < 4 {
				t.Fatalf("%s/%d: stream of %d bytes cannot even hold the end marker", kind, n, len(out))
			}
			switch magic := binary.LittleEndian.Uint32(out); magic {
			case magicUncompressed, magicCompressedV1, magicCompressedV2, magicEndOfStream:
			default:
				t.Fatalf("%s/%d: stream opens with %#x, which is not a block magic", kind, n, magic)
			}
			if end := binary.LittleEndian.Uint32(out[len(out)-4:]); end != magicEndOfStream {
				t.Fatalf("%s/%d: stream ends with %#x, want the end-of-stream marker", kind, n, end)
			}
		}
	}
}

// TestIncompressibleDataIsStoredPlainly checks the fallback: coding random
// bytes cannot pay for itself, so they are stored as they are rather than made
// larger.
func TestIncompressibleDataIsStoredPlainly(t *testing.T) {
	src := corpus("random", 200000, 2)
	out := Encode(nil, src)
	if magic := binary.LittleEndian.Uint32(out); magic != magicUncompressed {
		t.Fatalf("random data was coded as %#x rather than stored", magic)
	}
	if len(out) != len(src)+uncompressedBlockOverhead {
		t.Fatalf("stored %d bytes for a %d byte input", len(out), len(src))
	}
	if !bytes.Equal(out[8:len(out)-4], src) {
		t.Fatal("the stored bytes are not the input")
	}
}

func TestCompressibleDataShrinks(t *testing.T) {
	for _, kind := range []string{"zeros", "repeat", "text", "runs"} {
		src := corpus(kind, 200000, 3)
		out := Encode(nil, src)
		if len(out) >= len(src)/2 {
			t.Errorf("%s: %d bytes only came down to %d", kind, len(src), len(out))
		}
	}
}

// TestEncodeAppends checks that Encode adds to what it is given rather than
// replacing it, and does not disturb what is already there.
func TestEncodeAppends(t *testing.T) {
	prefix := []byte("keep me")
	src := corpus("text", 5000, 4)
	out := Encode(append([]byte(nil), prefix...), src)
	if !bytes.HasPrefix(out, prefix) {
		t.Fatal("Encode overwrote the destination")
	}
	if alone := Encode(nil, src); !bytes.Equal(out[len(prefix):], alone) {
		t.Fatal("appending changed the stream")
	}
}

// TestEncoderReuse checks that one Encoder gives the same answer for the same
// input however many streams it has coded before, which it only does if no
// state survives between them.
func TestEncoderReuse(t *testing.T) {
	e := NewEncoder()
	for _, kind := range kinds {
		src := corpus(kind, 50000, 5)
		want := Encode(nil, src)
		for range 3 {
			_ = e.Encode(nil, corpus("mixed", 70000, 6)) // Dirty the state.
			if got := e.Encode(nil, src); !bytes.Equal(got, want) {
				t.Fatalf("%s: a reused encoder produced a different stream", kind)
			}
		}
	}
}

// TestSymbolTablesTileTheValueRange checks that the symbol each value is given
// can actually express it: the value must not fall below the symbol's base, and
// the difference must fit the extra bits the symbol carries.
func TestSymbolTablesTileTheValueRange(t *testing.T) {
	for _, c := range []struct {
		name   string
		table  []uint8
		base   []int32
		extra  []uint8
		maxVal int32
	}{
		{"L", lSymbolOf, lBaseValue[:], lExtraBits[:], maxLValue},
		{"M", mSymbolOf, mBaseValue[:], mExtraBits[:], maxMValue},
		{"D", dSymbolOf, dBaseValue[:], dExtraBits[:], maxDValue},
	} {
		if len(c.table) != int(c.maxVal)+1 {
			t.Fatalf("%s: table covers %d values, want %d", c.name, len(c.table), c.maxVal+1)
		}
		for v := int32(0); v <= c.maxVal; v++ {
			s := c.table[v]
			delta := v - c.base[s]
			if delta < 0 {
				t.Fatalf("%s: value %d was given symbol %d, whose base is %d", c.name, v, s, c.base[s])
			}
			if delta >= 1<<c.extra[s] {
				t.Fatalf("%s: value %d needs %d beyond the base of symbol %d, which carries %d bits",
					c.name, v, delta, s, c.extra[s])
			}
		}
	}
}

// TestNormalizeFreqUsesEveryState checks the property the entropy tables rest
// on: the frequencies must sum to exactly the number of states, and a symbol
// that occurs must be given at least one, or it could not be coded.
func TestNormalizeFreqUsesEveryState(t *testing.T) {
	r := rand.New(rand.NewSource(9))
	// The four alphabets the format actually uses, each with at least as many
	// states as it has symbols.
	alphabets := []struct{ states, symbols int }{
		{lStates, lSymbols}, {mStates, mSymbols}, {dStates, dSymbols}, {litStates, litSymbols},
	}
	for _, a := range alphabets {
		states, symbols := a.states, a.symbols
		{
			for range 300 {
				occ := make([]uint32, symbols)
				switch r.Intn(4) {
				case 0: // Nothing occurs at all.
				case 1: // One symbol takes everything.
					occ[r.Intn(symbols)] = uint32(r.Intn(1 << 20))
				case 2: // A long tail of rare symbols.
					for i := range occ {
						if r.Intn(3) == 0 {
							occ[i] = uint32(r.Intn(4))
						}
					}
				default:
					for i := range occ {
						occ[i] = uint32(r.Intn(1 << 16))
					}
				}

				freq := make([]uint16, symbols)
				normalizeFreq(states, occ, freq)

				total := 0
				for i, f := range freq {
					total += int(f)
					if occ[i] != 0 && f == 0 {
						t.Fatalf("states=%d symbols=%d: symbol %d occurs %d times but was given no states",
							states, symbols, i, occ[i])
					}
				}
				used := false
				for _, n := range occ {
					used = used || n != 0
				}
				// States are only shared out among the symbols that occur. A
				// histogram with nothing in it is the exception: the leftover
				// goes to symbol zero, and nothing is ever coded against it.
				if used {
					for i, f := range freq {
						if occ[i] == 0 && f != 0 {
							t.Fatalf("states=%d symbols=%d: symbol %d never occurs but was given %d states",
								states, symbols, i, f)
						}
					}
				}
				if used && total != states {
					t.Fatalf("states=%d symbols=%d: frequencies sum to %d", states, symbols, total)
				}
			}
		}
	}
}

// TestReferenceRoundTrip decodes what this package produces with the reference
// implementation. It needs a decoder built from github.com/lzfse/lzfse, which
// reads a stream on its standard input and writes the decoded bytes out.
func TestReferenceRoundTrip(t *testing.T) {
	decoder := os.Getenv("LZFSE_REFERENCE_DECODER")
	if decoder == "" {
		t.Skip("set LZFSE_REFERENCE_DECODER to the path of a reference decoder")
	}
	e := NewEncoder()
	for _, kind := range kinds {
		for _, n := range append(sizes, 1<<20) {
			src := corpus(kind, n, 11)
			cmd := exec.Command(decoder)
			cmd.Stdin = bytes.NewReader(e.Encode(nil, src))
			var out, errb bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &errb
			if err := cmd.Run(); err != nil {
				t.Fatalf("%s/%d: the reference decoder rejected the stream: %v %s", kind, n, err, errb.String())
			}
			if !bytes.Equal(out.Bytes(), src) {
				t.Fatalf("%s/%d: decoded %d bytes, want %d", kind, n, out.Len(), len(src))
			}
		}
	}
}

// TestEmptyInput pins down the shape of a stream with nothing in it: there is
// no block to write, so only the end marker is left.
func TestEmptyInput(t *testing.T) {
	out := Encode(nil, nil)
	if len(out) != 4 || binary.LittleEndian.Uint32(out) != magicEndOfStream {
		t.Fatalf("an empty input gave %x", out)
	}
}
