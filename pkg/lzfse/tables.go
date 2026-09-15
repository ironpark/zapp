package lzfse

// Block magics. A stream is a sequence of blocks ending with the end-of-stream
// magic, and each block says how it was coded.
const (
	magicEndOfStream    = 0x24787662 // bvx$
	magicUncompressed   = 0x2d787662 // bvx-
	magicCompressedV1   = 0x31787662 // bvx1, tables stored plainly
	magicCompressedV2   = 0x32787662 // bvx2, tables stored compressed
	magicCompressedLZVN = 0x6e787662 // bvxn
)

// Sizes of the four coded streams: how many symbols each alphabet has, and how
// many states its entropy table is given.
const (
	lSymbols   = 20
	mSymbols   = 20
	dSymbols   = 64
	litSymbols = 256

	lStates   = 64
	mStates   = 64
	dStates   = 256
	litStates = 1024
)

// The largest literal run, match length, and match distance the symbol tables
// can express. Anything longer has to be split across several matches.
const (
	maxLValue = 315
	maxMValue = 2359
	maxDValue = 262139
)

// How much a block holds before it is emitted. The literal count is bounded
// because the header records it in 20 bits, and because the encoder works from
// fixed buffers.
const (
	matchesPerBlock  = 10000
	literalsPerBlock = 4 * matchesPerBlock
)

// Each value is coded as a symbol naming a base, plus a number of extra bits
// holding the difference from it.
var (
	lExtraBits = [lSymbols]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 3, 5, 8}
	lBaseValue = [lSymbols]int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 20, 28, 60}

	mExtraBits = [mSymbols]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3, 5, 8, 11}
	mBaseValue = [mSymbols]int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 24, 56, 312}

	dExtraBits = [dSymbols]uint8{
		0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3,
		4, 4, 4, 4, 5, 5, 5, 5, 6, 6, 6, 6, 7, 7, 7, 7,
		8, 8, 8, 8, 9, 9, 9, 9, 10, 10, 10, 10, 11, 11, 11, 11,
		12, 12, 12, 12, 13, 13, 13, 13, 14, 14, 14, 14, 15, 15, 15, 15,
	}
	dBaseValue = [dSymbols]int32{
		0, 1, 2, 3, 4, 6, 8, 10, 12, 16,
		20, 24, 28, 36, 44, 52, 60, 76, 92, 108,
		124, 156, 188, 220, 252, 316, 380, 444, 508, 636,
		764, 892, 1020, 1276, 1532, 1788, 2044, 2556, 3068, 3580,
		4092, 5116, 6140, 7164, 8188, 10236, 12284, 14332, 16380, 20476,
		24572, 28668, 32764, 40956, 49148, 57340, 65532, 81916, 98300, 114684,
		131068, 163836, 196604, 229372,
	}
)

// Symbol lookups, built once from the base tables above rather than stored, so
// that the bases remain the only statement of the mapping.
var (
	lSymbolOf = buildSymbolTable(maxLValue, lBaseValue[:])
	mSymbolOf = buildSymbolTable(maxMValue, mBaseValue[:])
	dSymbolOf = buildSymbolTable(maxDValue, dBaseValue[:])
)

// buildSymbolTable maps every value up to maxValue to the last symbol whose
// base does not exceed it.
func buildSymbolTable(maxValue int32, base []int32) []uint8 {
	table := make([]uint8, maxValue+1)
	symbol := 0
	for v := int32(0); v <= maxValue; v++ {
		for symbol+1 < len(base) && base[symbol+1] <= v {
			symbol++
		}
		table[v] = uint8(symbol)
	}
	return table
}
