package apfs

import (
	"encoding/binary"
	"hash/crc32"
)

// APFS pins normalization and full case folding to Unicode 9. Updating Go's
// Unicode version must not change the hash or ordering of directory records.
func normalizedName(s string, fold bool) string {
	if fold {
		// Order combining marks before folding: U+0345 folds to a spacing
		// iota, which otherwise prematurely ends its combining sequence.
		var folded []rune
		for _, r := range normalizedName(s, false) {
			if f, ok := caseFold[r]; ok {
				folded = append(folded, []rune(f)...)
			} else {
				folded = append(folded, r)
			}
		}
		return normalizedName(string(folded), false)
	}
	var out []rune
	var decompose func(rune)
	decompose = func(r rune) {
		if r >= 0xac00 && r < 0xac00+11172 {
			n := r - 0xac00
			out = append(out, 0x1100+n/588, 0x1161+(n%588)/28)
			if t := n % 28; t != 0 {
				out = append(out, 0x11a7+t)
			}
		} else if d, ok := decomposition[r]; ok {
			for _, c := range d {
				decompose(c)
			}
		} else {
			out = append(out, r)
		}
	}
	for _, r := range s {
		decompose(r)
	}
	// Stable canonical ordering within each combining sequence.
	for i := 1; i < len(out); i++ {
		cc := combiningClass[out[i]]
		if cc == 0 {
			continue
		}
		for j := i; j > 0 && combiningClass[out[j-1]] > cc; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return string(out)
}

var castagnoli = crc32.MakeTable(crc32.Castagnoli)

func nameHash(name string, fold bool) uint32 {
	var b []byte
	for _, r := range normalizedName(name, fold) {
		b = binary.LittleEndian.AppendUint32(b, uint32(r))
	}
	// The terminator contributes to name_len, but not to the hash.
	return (^crc32.Checksum(b, castagnoli)) & 0x3fffff
}
