// Package lzfse compresses data in Apple's LZFSE format, following the
// reference implementation at github.com/lzfse/lzfse. Compression only;
// decompression is left to the system, which is what reads these streams.
//
// A stream is a sequence of blocks ending with an end-of-stream marker. Each
// block is either stored as it is, or coded as a stream of literals and of
// (literal count, match length, match distance) triples, each entropy coded
// with finite state entropy against frequency tables carried in the block.
package lzfse

import "encoding/binary"

// Encoder holds the working buffers for compression. A single Encoder may be
// reused for any number of streams, which avoids reallocating the history
// table for each one, but it may not be used from two goroutines at once.
type Encoder struct {
	src        []byte
	out        []byte
	srcLiteral int

	lValues  []uint32
	mValues  []uint32
	dValues  []uint32
	literals []byte
	nMatches int
	nLits    int

	history []historyEntry
	pending match

	lFreq   [lSymbols]uint16
	mFreq   [mSymbols]uint16
	dFreq   [dSymbols]uint16
	litFreq [litSymbols]uint16

	lEnc   [lSymbols]encoderEntry
	mEnc   [mSymbols]encoderEntry
	dEnc   [dSymbols]encoderEntry
	litEnc [litSymbols]encoderEntry
}

// NewEncoder returns an Encoder with its buffers allocated.
func NewEncoder() *Encoder {
	return &Encoder{
		lValues: make([]uint32, matchesPerBlock),
		mValues: make([]uint32, matchesPerBlock),
		dValues: make([]uint32, matchesPerBlock),
		// Literals are copied in 16 byte runs, so the buffer carries enough
		// slack for the last run to overshoot.
		literals: make([]byte, literalsPerBlock+16),
		history:  make([]historyEntry, hashValues),
	}
}

// Encode appends the compressed form of src to dst and returns the result.
func Encode(dst, src []byte) []byte { return NewEncoder().Encode(dst, src) }

// Encode appends the compressed form of src to dst and returns the result.
func (e *Encoder) Encode(dst, src []byte) []byte {
	start := len(dst)
	e.reset(src, dst)
	e.compress()

	// Compression that did not pay for itself is not worth the decoder's time.
	if len(e.out)-start >= len(src)+uncompressedBlockOverhead {
		return appendUncompressed(dst[:start], src)
	}
	return e.out
}

// uncompressedBlockOverhead is what storing data plainly costs: a block header
// and the end-of-stream marker.
const uncompressedBlockOverhead = 8 + 4

// appendUncompressed stores src without coding it, which is always valid and
// is what incompressible data ends up as.
func appendUncompressed(dst, src []byte) []byte {
	dst = binary.LittleEndian.AppendUint32(dst, magicUncompressed)
	dst = binary.LittleEndian.AppendUint32(dst, uint32(len(src)))
	dst = append(dst, src...)
	return binary.LittleEndian.AppendUint32(dst, magicEndOfStream)
}

func (e *Encoder) reset(src, out []byte) {
	e.src = src
	e.out = out
	e.srcLiteral = 0
	e.nMatches = 0
	e.nLits = 0
	e.pending = match{}

	var empty historyEntry
	for i := range empty.pos {
		empty.pos[i] = invalidPos
	}
	for i := range e.history {
		e.history[i] = empty
	}
}

// compress runs the match finder over the whole input and emits the blocks it
// produces, followed by the end-of-stream marker.
func (e *Encoder) compress() {
	e.findMatches()
	if e.pending.length > 0 {
		e.pushMatch(e.pending)
		e.pending = match{}
	}
	if trailing := len(e.src) - e.srcLiteral; trailing > 0 {
		e.pushLiterals(trailing)
	}
	e.emitBlock()
	e.out = binary.LittleEndian.AppendUint32(e.out, magicEndOfStream)
}
