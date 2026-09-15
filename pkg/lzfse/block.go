package lzfse

// pushLiterals records n bytes that are not covered by any match, splitting
// the run into as many triples as the literal count range allows. The distance
// recorded goes unused when there is no match, and one codes smallest.
func (e *Encoder) pushLiterals(n int) {
	for n > maxLValue {
		e.pushLMD(maxLValue, 0, 1)
		n -= maxLValue
	}
	if n > 0 {
		e.pushLMD(uint32(n), 0, 1)
	}
}

// pushMatch splits a match into as many coded triples as the value ranges
// require and records them.
func (e *Encoder) pushMatch(m match) {
	l := uint32(m.pos - e.srcLiteral)
	length := uint32(m.length)
	d := uint32(m.pos - m.ref)

	for l > maxLValue {
		e.pushLMD(maxLValue, 0, 1)
		l -= maxLValue
	}
	for length > maxMValue {
		e.pushLMD(l, maxMValue, d)
		l, length = 0, length-maxMValue
	}
	if l > 0 || length > 0 {
		e.pushLMD(l, length, d)
	}
}

// pushLMD records one triple and the literals that precede it, closing the
// current block first if it has no room left. Both counts must already be
// within the ranges the symbol tables can express.
func (e *Encoder) pushLMD(l, m, d uint32) {
	if e.nMatches+1 > matchesPerBlock || e.nLits+int(l)+4 > literalsPerBlock {
		e.emitBlock()
	}
	e.lValues[e.nMatches] = l
	e.mValues[e.nMatches] = m
	e.dValues[e.nMatches] = d
	e.nMatches++

	copy(e.literals[e.nLits:], e.src[e.srcLiteral:e.srcLiteral+int(l)])
	e.nLits += int(l)
	e.srcLiteral += int(l) + int(m)
}

// emitBlock codes everything recorded so far as one compressed block.
func (e *Encoder) emitBlock() {
	if e.nMatches == 0 && e.nLits == 0 {
		return
	}
	// The literals are coded as four interleaved streams, so their number is
	// rounded up to a multiple of four.
	for e.nLits&3 != 0 {
		e.literals[e.nLits] = 0
		e.nLits++
	}
	// A repeated distance is coded as zero, which is both shorter and more
	// predictable than the distance itself.
	prev := uint32(0)
	for i := range e.nMatches {
		if d := e.dValues[i]; d == prev {
			e.dValues[i] = 0
		} else {
			prev = d
		}
	}

	var lOcc [lSymbols]uint32
	var mOcc [mSymbols]uint32
	var dOcc [dSymbols]uint32
	var litOcc [litSymbols]uint32
	var rawBytes uint32
	for i := range e.nMatches {
		l, m := e.lValues[i], e.mValues[i]
		rawBytes += l + m
		lOcc[lSymbolOf[l]]++
		mOcc[mSymbolOf[m]]++
		dOcc[dSymbolOf[e.dValues[i]]]++
	}
	for _, lit := range e.literals[:e.nLits] {
		litOcc[lit]++
	}

	normalizeFreq(lStates, lOcc[:], e.lFreq[:])
	normalizeFreq(mStates, mOcc[:], e.mFreq[:])
	normalizeFreq(dStates, dOcc[:], e.dFreq[:])
	normalizeFreq(litStates, litOcc[:], e.litFreq[:])

	initEncoderTable(lStates, e.lFreq[:], e.lEnc[:])
	initEncoderTable(mStates, e.mFreq[:], e.mEnc[:])
	initEncoderTable(dStates, e.dFreq[:], e.dEnc[:])
	initEncoderTable(litStates, e.litFreq[:], e.litEnc[:])

	e.writeBlock(rawBytes)

	e.nMatches = 0
	e.nLits = 0
}
