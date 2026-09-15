//go:build !goexperiment.simd

package lzfse

// matchLength returns how far the bytes at ref and pos agree, starting from
// four bytes already known to be equal. Without the simd experiment enabled,
// the comparison runs eight bytes at a time on plain integers.
func matchLength(src []byte, ref, pos, maxLength int) int {
	return matchLengthScalar(src, ref, pos, maxLength)
}
