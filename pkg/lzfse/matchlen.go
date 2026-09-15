package lzfse

import (
	"encoding/binary"
	"math/bits"
)

// matchLengthScalar compares eight bytes at a time, stopping at the first byte
// that differs. The caller has already established that four bytes match and
// that reading eight bytes past maxLength stays inside the input.
func matchLengthScalar(src []byte, ref, pos, maxLength int) int {
	return matchLengthFrom(src, ref, pos, maxLength, 4)
}

// matchLengthFrom continues a comparison that is already known to agree for
// the first length bytes.
func matchLengthFrom(src []byte, ref, pos, maxLength, length int) int {
	for length < maxLength {
		d := binary.LittleEndian.Uint64(src[ref+length:]) ^ binary.LittleEndian.Uint64(src[pos+length:])
		if d == 0 {
			length += 8
			continue
		}
		// The first set bit marks the first byte that differs.
		return length + bits.TrailingZeros64(d)>>3
	}
	return length
}
