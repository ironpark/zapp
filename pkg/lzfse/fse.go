package lzfse

import "math/bits"

// Finite state entropy coding, the "FSE" in LZFSE. A symbol is coded by the
// number of bits its probability warrants, by keeping a state that walks a
// table built from the symbol frequencies. Encoding runs backwards over the
// symbols so that decoding can run forwards.

// encoderEntry describes how one symbol moves the state.
type encoderEntry struct {
	s0     int16 // First state needing a k-bit shift; below it, k-1 bits.
	k      int16
	delta0 int16 // Increment for states at or above s0,
	delta1 int16 // and for those below it.
}

// normalizeFreq rescales a histogram so that the frequencies sum to exactly
// nstates, which must be a power of two. A symbol that occurs at all is always
// given at least one state, since a symbol with no states cannot be coded.
//
// It follows that no more symbols may occur than there are states. Every
// alphabet in this format has at least as many states as symbols, so the
// condition holds by construction.
func normalizeFreq(nstates int, occ []uint32, freq []uint16) {
	var total uint32
	for _, n := range occ {
		total += n
	}
	var step uint32
	if total != 0 {
		step = (uint32(1) << 31) / total
	}
	shift := bits.LeadingZeros32(uint32(nstates)) - 1

	remaining := nstates
	maxFreq, maxSym := 0, 0
	for i, n := range occ {
		// Scale and round to nearest, in integer arithmetic.
		f := int(((n*step)>>shift)+1) >> 1
		if f == 0 && n != 0 {
			f = 1
		}
		freq[i] = uint16(f)
		remaining -= f
		if f > maxFreq {
			maxFreq, maxSym = f, i
		}
	}

	switch {
	case remaining == 0:
	case -remaining < maxFreq>>2:
		// Whatever is left over, or a small excess, is absorbed by the symbol
		// that can best afford it.
		freq[maxSym] = uint16(int(freq[maxSym]) + remaining)
	default:
		adjustFreqs(freq, -remaining)
	}
}

// adjustFreqs takes states back from the symbols that have the most of them,
// in decreasing proportions, until the excess is gone.
func adjustFreqs(freq []uint16, overrun int) {
	for shift := 3; overrun != 0; shift-- {
		if shift < 0 {
			// Every symbol is down to its last state and the excess is still
			// there, so more symbols occur than there are states to code them
			// with. normalizeFreq documents why that cannot arise; stopping
			// here keeps the impossible case from spinning.
			return
		}
		for sym := range freq {
			if freq[sym] <= 1 {
				continue
			}
			n := int(freq[sym]-1) >> shift
			if n > overrun {
				n = overrun
			}
			freq[sym] -= uint16(n)
			overrun -= n
			if overrun == 0 {
				break
			}
		}
	}
}

// initEncoderTable builds the per-symbol table the encoder walks. States are
// handed out to symbols in order, each symbol taking as many as its frequency.
func initEncoderTable(nstates int, freq []uint16, table []encoderEntry) {
	offset := 0
	nclz := bits.LeadingZeros32(uint32(nstates))
	for i, f := range freq {
		if f == 0 {
			continue // The symbol does not occur, so it needs no states.
		}
		// k is the shift that puts f between nstates and 2*nstates.
		k := bits.LeadingZeros32(uint32(f)) - nclz
		entry := encoderEntry{
			s0:     int16(int(f)<<k - nstates),
			k:      int16(k),
			delta0: int16(offset - int(f) + nstates>>k),
		}
		// A symbol holding every state has k == 0 and s0 == 0, so no state is
		// ever below s0 and the k-1 branch is unreachable. Its increment is
		// left at zero rather than computed from a negative shift.
		if k > 0 {
			entry.delta1 = int16(offset - int(f) + nstates>>(k-1))
		}
		table[i] = entry
		offset += int(f)
	}
}

// outStream accumulates bits least significant first, spilling whole bytes to
// the output as it goes.
type outStream struct {
	accum  uint64
	nbits  int
	buf    []byte
	offset int
}

// push adds the low n bits of b.
func (o *outStream) push(n int, b uint64) {
	o.accum |= b << o.nbits
	o.nbits += n
}

// flush writes out every whole byte held, leaving fewer than eight bits behind.
func (o *outStream) flush() {
	n := o.nbits &^ 7
	o.store()
	o.offset += n >> 3
	o.accum >>= n
	o.nbits -= n
}

// finish writes out the remaining bits, padding the last byte with zeroes. It
// leaves nbits in [-7, 0], which the block header records so that a decoder
// knows how many of those padding bits to discard.
func (o *outStream) finish() {
	n := (o.nbits + 7) &^ 7
	o.store()
	o.offset += n >> 3
	o.accum = 0
	o.nbits -= n
}

// store writes the accumulator at the current offset. Eight bytes always go
// out; the offset only advances over the ones that are complete, so the rest
// are overwritten by the next store.
func (o *outStream) store() {
	for i := range 8 {
		o.buf[o.offset+i] = byte(o.accum >> (8 * i))
	}
}

// encode codes one symbol, writing the bits of the current state that the new
// state has no room for.
func (o *outStream) encode(state *int, table []encoderEntry, symbol uint8) {
	e := table[symbol]
	s := *state
	nbits, delta := int(e.k), int(e.delta0)
	if s < int(e.s0) {
		nbits, delta = nbits-1, int(e.delta1)
	}
	o.push(nbits, uint64(s)&(1<<nbits-1))
	*state = delta + s>>nbits
}
