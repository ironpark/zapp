package lzfse

import "encoding/binary"

// The match finder keeps, for each hash of four bytes, the few most recent
// places those bytes were seen, along with the bytes themselves so that a
// candidate can be rejected without touching the input again.
const (
	hashBits   = 14
	hashValues = 1 << hashBits
	hashWidth  = 4

	// goodMatch is the length at which a match is taken immediately rather
	// than held back to see whether the next position does better.
	goodMatch = 40

	// maxMatchLength bounds a single match so that it does not split into an
	// unreasonable number of coded triples.
	maxMatchLength = 100 * maxMValue
)

// invalidPos stands for "nothing recorded here yet". Zero will not do: it is a
// real position, and an empty table would otherwise offer position zero as a
// candidate for any four bytes that happen to be zero. It sits far enough back
// that the distance check rejects it.
const invalidPos = -4 * maxDValue

type historyEntry struct {
	pos   [hashWidth]int32
	value [hashWidth]uint32
}

// match is one back reference: length bytes at pos repeat what is at ref.
type match struct {
	pos    int
	ref    int
	length int
}

// hash4 mixes four bytes down to a hash table index.
func hash4(x uint32) uint32 {
	x = (x * 2654435761) >> (32 - hashBits)
	return x & (hashValues - 1)
}

// findMatches walks the input, records where each four byte sequence was seen,
// and emits the matches it finds. A match is held back one position to see
// whether the next position starts a longer one, which is what keeps a short
// match from displacing a better one that overlaps it.
func (e *Encoder) findMatches() {
	src := e.src
	if len(src) < 8 {
		return
	}
	// Match extension reads eight bytes at a time, so the search stops short
	// of the end and the tail is emitted as literals.
	end := len(src) - 8

	for pos := 0; pos < end; pos++ {
		x := binary.LittleEndian.Uint32(src[pos:])
		line := &e.history[hash4(x)]

		// Positions already covered by a match that has been emitted are not
		// searched again, but every position is still recorded.
		var best match
		if pos >= e.srcLiteral {
			best = e.bestMatch(line, x, pos)
		}

		// Record this position, dropping the oldest of the set. The entries
		// are shifted in place: building a new set and assigning it copies the
		// whole entry twice over, on every byte of the input.
		for k := hashWidth - 1; k > 0; k-- {
			line.pos[k], line.value[k] = line.pos[k-1], line.value[k-1]
		}
		line.pos[0], line.value[0] = int32(pos), x

		if best.length == 0 {
			e.flushStaleLiterals(pos)
			continue
		}
		if best.length > maxMatchLength {
			best.length = maxMatchLength
		}
		// Reach backwards from the match as far as the bytes still agree,
		// which turns literals into match at no cost.
		for best.pos > e.srcLiteral && best.ref > 0 && src[best.ref-1] == src[best.pos-1] {
			best.pos--
			best.ref--
			best.length++
		}

		switch {
		case best.length >= goodMatch:
			e.pushMatch(best)
			e.pending = match{}
		case e.pending.length == 0:
			e.pending = best
		case e.pending.pos+e.pending.length <= best.pos:
			// The two do not overlap, so both are worth having.
			e.pushMatch(e.pending)
			e.pending = best
		case best.length > e.pending.length:
			e.pushMatch(best)
			e.pending = match{}
		default:
			e.pushMatch(e.pending)
			e.pending = match{}
		}
	}
}

// bestMatch returns the longest match at pos among the recorded candidates.
func (e *Encoder) bestMatch(seen *historyEntry, x uint32, pos int) match {
	best := match{pos: pos}
	// Extension may read eight bytes past the length being tested, which the
	// search bound already keeps inside the input.
	maxLength := len(e.src) - pos - 8
	for k := range hashWidth {
		if seen.value[k] != x {
			continue // The first four bytes differ, so there is no match.
		}
		ref := int(seen.pos[k])
		if ref+maxDValue < pos || ref >= pos {
			continue // Too far back to code, or not yet written.
		}
		length := matchLength(e.src, ref, pos, maxLength)
		if length > best.length {
			best.length, best.ref = length, ref
		}
	}
	return best
}

// flushStaleLiterals emits literals when the search has run far ahead of them,
// so that a long run without matches cannot overflow the block's literal
// buffer before the block is closed.
func (e *Encoder) flushStaleLiterals(pos int) {
	if pos-e.srcLiteral <= 8*maxLValue {
		return
	}
	if e.pending.length > 0 {
		e.pushMatch(e.pending)
		e.pending = match{}
		return
	}
	e.pushLiterals(maxLValue)
}
