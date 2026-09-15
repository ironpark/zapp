package lzfse

import "encoding/binary"

// v2HeaderFixedSize is the part of a compressed block header that is always
// present: the magic, the decoded size, and three packed fields. The coded
// frequency tables follow, and their length is what varies.
const v2HeaderFixedSize = 4 + 4 + 3*8

// writeBlock serialises one compressed block: the header carrying the
// frequency tables, then the literals, then the triples.
func (e *Encoder) writeBlock(rawBytes uint32) {
	freq := e.encodeFreqTables()
	headerSize := v2HeaderFixedSize + len(freq)

	// Both payloads are written through an accumulator that stores eight bytes
	// at a time, so the buffer carries slack past whatever they occupy. Every
	// byte kept is written before the block is truncated to its real length,
	// so the slack is left as it is rather than cleared.
	bound := headerSize + 2*e.nLits + 8*e.nMatches + 128
	base := len(e.out)
	if cap(e.out)-base < bound {
		grown := make([]byte, base, 2*(base+bound))
		copy(grown, e.out)
		e.out = grown
	}
	buf := e.out[:base+bound]

	binary.LittleEndian.PutUint32(buf[base:], magicCompressedV2)
	binary.LittleEndian.PutUint32(buf[base+4:], rawBytes)
	copy(buf[base+v2HeaderFixedSize:], freq)

	litStart := base + headerSize
	litStates, litBits, litEnd := e.writeLiterals(buf, litStart)
	lmdStart := litEnd
	lmdState, lmdBits, lmdEnd := e.writeLMD(buf, lmdStart)

	// The three packed fields hold everything a decoder needs that is not a
	// frequency table. Bit positions follow the reference implementation.
	var packed [3]uint64
	packed[0] = field(uint32(e.nLits), 0, 20) |
		field(uint32(litEnd-litStart), 20, 20) |
		field(uint32(e.nMatches), 40, 20) |
		field(uint32(7+litBits), 60, 3)
	packed[1] = field(uint32(litStates[0]), 0, 10) |
		field(uint32(litStates[1]), 10, 10) |
		field(uint32(litStates[2]), 20, 10) |
		field(uint32(litStates[3]), 30, 10) |
		field(uint32(lmdEnd-lmdStart), 40, 20) |
		field(uint32(7+lmdBits), 60, 3)
	packed[2] = field(uint32(headerSize), 0, 32) |
		field(uint32(lmdState[0]), 32, 10) |
		field(uint32(lmdState[1]), 42, 10) |
		field(uint32(lmdState[2]), 52, 10)
	for i, v := range packed {
		binary.LittleEndian.PutUint64(buf[base+8+8*i:], v)
	}

	e.out = buf[:lmdEnd]
}

// field places v at a bit offset within a packed header word.
func field(v uint32, offset, nbits int) uint64 {
	return uint64(v) << offset
}

// writeLiterals codes the literals as four interleaved streams. They are coded
// from the last to the first, which is what lets a decoder read them forwards.
func (e *Encoder) writeLiterals(buf []byte, start int) (states [4]int, bits, end int) {
	out := outStream{buf: buf, offset: start}
	for i := e.nLits; i > 0; {
		i -= 4
		out.encode(&states[3], e.litEnc[:], e.literals[i+3])
		out.encode(&states[2], e.litEnc[:], e.literals[i+2])
		out.encode(&states[1], e.litEnc[:], e.literals[i+1])
		out.encode(&states[0], e.litEnc[:], e.literals[i+0])
		out.flush()
	}
	out.finish()
	return states, out.nbits, out.offset
}

// writeLMD codes the triples, again from last to first. Each value contributes
// its symbol and the extra bits holding its distance from that symbol's base.
func (e *Encoder) writeLMD(buf []byte, start int) (states [3]int, bits, end int) {
	// The payload opens with eight bytes of padding, which gives a decoder
	// room to prime its accumulator before the first symbol.
	for i := range 8 {
		buf[start+i] = 0
	}
	out := outStream{buf: buf, offset: start + 8}

	for i := e.nMatches - 1; i >= 0; i-- {
		d := e.dValues[i]
		ds := dSymbolOf[d]
		out.push(int(dExtraBits[ds]), uint64(int32(d)-dBaseValue[ds]))
		out.encode(&states[2], e.dEnc[:], ds)

		m := e.mValues[i]
		ms := mSymbolOf[m]
		out.push(int(mExtraBits[ms]), uint64(int32(m)-mBaseValue[ms]))
		out.encode(&states[1], e.mEnc[:], ms)

		l := e.lValues[i]
		ls := lSymbolOf[l]
		out.push(int(lExtraBits[ls]), uint64(int32(l)-lBaseValue[ls]))
		out.encode(&states[0], e.lEnc[:], ls)

		out.flush()
	}
	out.finish()
	return states, out.nbits, out.offset
}

// encodeFreqTables codes the four frequency tables with the fixed prefix code
// the format uses, where the position of the first zero bit gives the length.
func (e *Encoder) encodeFreqTables() []byte {
	out := make([]byte, 0, 2*(lSymbols+mSymbols+dSymbols+litSymbols))
	var accum uint32
	var nbits int

	appendValue := func(v uint16) {
		bits, n := encodeFreqValue(int(v))
		accum |= bits << nbits
		nbits += n
		for nbits >= 8 {
			out = append(out, byte(accum))
			accum >>= 8
			nbits -= 8
		}
	}
	for _, f := range e.lFreq {
		appendValue(f)
	}
	for _, f := range e.mFreq {
		appendValue(f)
	}
	for _, f := range e.dFreq {
		appendValue(f)
	}
	for _, f := range e.litFreq {
		appendValue(f)
	}
	if nbits > 0 {
		out = append(out, byte(accum))
	}
	return out
}

// encodeFreqValue returns the bits coding one frequency, and how many of them
// there are. Small values get short codes; the rest carry their value in a
// fixed width field after a prefix.
func encodeFreqValue(value int) (bits uint32, nbits int) {
	switch value {
	case 0:
		return 0, 2
	case 1:
		return 2, 2
	case 2:
		return 1, 3
	case 3:
		return 5, 3
	case 4:
		return 3, 5
	case 5:
		return 11, 5
	case 6:
		return 19, 5
	case 7:
		return 27, 5
	}
	if value < 24 {
		return uint32(7 + (value-8)<<4), 8
	}
	return uint32((value-24)<<4 + 15), 14
}
