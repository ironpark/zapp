//go:build goexperiment.simd

// Match extension through the experimental simd package, built only when that
// experiment is enabled; the default build uses the word-at-a-time version
// alongside this one.
//
// Measured on arm64, where a vector is 128 bits, this is about 5% slower than
// comparing 64 bits at a time. Two things work against it. Finding which byte
// of a vector differs means storing the vector and scanning its lanes, because
// the portable API has no way to ask a mask where its first set bit is. And a
// function holding vector code is too large for the inliner, while this one is
// called several million times per megabyte, so the call itself costs more than
// the wider comparison saves. A machine with 512-bit vectors compares eight
// times as much per step and may come out differently; that has not been
// measured here.
//
// The hot loop of this encoder is not here in any case: it is the history table
// update in match.go, which slides a set of entries along by one. The portable
// simd package offers no lane permute, so that operation cannot be expressed
// with it at all.

package lzfse

import (
	"encoding/binary"
	"math/bits"
	"simd"
)

// maxVectorLanes is the most 64-bit lanes a vector can hold, which is what a
// 512-bit register gives.
const maxVectorLanes = 8

// scalarRun is how far the comparison goes a word at a time before widening.
// Most matches end within the first few bytes, and this function is called
// several million times per megabyte, so the common case has to stay small
// enough to inline into the caller; only a match that has already run this far
// is worth the call to the vector path.
const scalarRun = 16

// matchLength returns how far the bytes at ref and pos agree, starting from
// four bytes already known to be equal. Only the first word is compared here,
// which is what almost every call needs and is small enough to inline; the
// rest is left to a function that can afford to be larger.
func matchLength(src []byte, ref, pos, maxLength int) int {
	if maxLength > 4 {
		d := binary.LittleEndian.Uint64(src[ref+4:]) ^ binary.LittleEndian.Uint64(src[pos+4:])
		if d != 0 {
			return 4 + bits.TrailingZeros64(d)>>3
		}
	}
	return matchLengthWide(src, ref, pos, maxLength, 4)
}

// matchLengthWide continues a long match a vector at a time. It is kept apart
// from matchLength so that the hot path above remains inlinable.
func matchLengthWide(src []byte, ref, pos, maxLength, length int) int {
	if length = matchLengthFrom(src, ref, pos, min(maxLength, scalarRun), length); length < scalarRun || length >= maxLength {
		return length
	}
	var probe simd.Uint8s
	width := probe.Len()
	var lanes [maxVectorLanes]uint64
	for length+width <= maxLength {
		d := simd.LoadUint8s(src[ref+length:]).ReshapeToUint64s().
			Xor(simd.LoadUint8s(src[pos+length:]).ReshapeToUint64s())
		n := d.Len()
		d.Store(lanes[:n])
		for i, word := range lanes[:n] {
			if word != 0 {
				// The first set bit marks the first byte that differs.
				return length + 8*i + bits.TrailingZeros64(word)>>3
			}
		}
		length += width
	}
	return matchLengthFrom(src, ref, pos, maxLength, length)
}
