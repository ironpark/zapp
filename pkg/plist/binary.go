package plist

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
	"unicode/utf16"
)

// Binary property lists ("bplist00") are laid out as a header, a stream of
// objects, an offset table locating each object, and a 32-byte trailer that
// describes the table. Object graphs are read lazily from the top object,
// following integer references into the offset table.

const (
	bplistMagic      = "bplist00"
	bplistTrailerLen = 32
)

type bplistReader struct {
	data          []byte
	offsets       []uint64
	objectRefSize uint8
	// visiting guards against the reference cycles a hand-crafted file can
	// encode, which would otherwise recurse until the stack is exhausted.
	visiting map[uint64]bool
}

func isBinary(data []byte) bool {
	return len(data) >= len(bplistMagic) && string(data[:len(bplistMagic)]) == bplistMagic
}

func parseBinary(data []byte) (interface{}, error) {
	if len(data) < len(bplistMagic)+bplistTrailerLen {
		return nil, fmt.Errorf("binary plist is too short")
	}
	trailer := data[len(data)-bplistTrailerLen:]
	offsetIntSize := trailer[6]
	objectRefSize := trailer[7]
	numObjects := binary.BigEndian.Uint64(trailer[8:16])
	topObject := binary.BigEndian.Uint64(trailer[16:24])
	tableOffset := binary.BigEndian.Uint64(trailer[24:32])

	if offsetIntSize < 1 || offsetIntSize > 8 || objectRefSize < 1 || objectRefSize > 8 {
		return nil, fmt.Errorf("binary plist has an invalid trailer")
	}
	tableLen := numObjects * uint64(offsetIntSize)
	if tableOffset > uint64(len(data)) || tableLen > uint64(len(data))-tableOffset {
		return nil, fmt.Errorf("binary plist offset table is out of range")
	}
	if topObject >= numObjects {
		return nil, fmt.Errorf("binary plist top object is out of range")
	}

	r := &bplistReader{
		data:          data,
		offsets:       make([]uint64, numObjects),
		objectRefSize: objectRefSize,
		visiting:      map[uint64]bool{},
	}
	table := data[tableOffset : tableOffset+tableLen]
	for i := range r.offsets {
		r.offsets[i] = beUint(table[i*int(offsetIntSize) : (i+1)*int(offsetIntSize)])
	}
	return r.object(topObject)
}

// beUint reads a big-endian unsigned integer of 1..8 bytes.
func beUint(b []byte) uint64 {
	var n uint64
	for _, c := range b {
		n = n<<8 | uint64(c)
	}
	return n
}

func (r *bplistReader) object(ref uint64) (interface{}, error) {
	if ref >= uint64(len(r.offsets)) {
		return nil, fmt.Errorf("binary plist object reference %d is out of range", ref)
	}
	if r.visiting[ref] {
		return nil, fmt.Errorf("binary plist contains a reference cycle")
	}
	r.visiting[ref] = true
	defer delete(r.visiting, ref)

	off := r.offsets[ref]
	if off >= uint64(len(r.data)) {
		return nil, fmt.Errorf("binary plist object %d is out of range", ref)
	}
	marker := r.data[off]
	kind, arg := marker>>4, uint64(marker&0x0f)
	body := off + 1

	switch kind {
	case 0x0:
		switch arg {
		case 0x0:
			return nil, nil
		case 0x8:
			return false, nil
		case 0x9:
			return true, nil
		case 0xf: // fill byte; no value of its own
			return nil, nil
		}
		return nil, fmt.Errorf("unsupported binary plist marker 0x%02x", marker)

	case 0x1: // integer, 2^arg bytes
		size := uint64(1) << arg
		raw, err := r.slice(body, size)
		if err != nil {
			return nil, err
		}
		switch {
		case size < 8:
			return int64(beUint(raw)), nil
		case size == 8:
			return int64(binary.BigEndian.Uint64(raw)), nil
		default:
			// 128-bit integers are stored sign-extended; keep the low 64 bits.
			return int64(binary.BigEndian.Uint64(raw[size-8:])), nil
		}

	case 0x2: // real, 2^arg bytes
		size := uint64(1) << arg
		raw, err := r.slice(body, size)
		if err != nil {
			return nil, err
		}
		switch size {
		case 4:
			return float64(math.Float32frombits(binary.BigEndian.Uint32(raw))), nil
		case 8:
			return math.Float64frombits(binary.BigEndian.Uint64(raw)), nil
		}
		return nil, fmt.Errorf("unsupported binary plist real of %d bytes", size)

	case 0x3: // date, always 8 bytes of CFAbsoluteTime
		raw, err := r.slice(body, 8)
		if err != nil {
			return nil, err
		}
		secs := math.Float64frombits(binary.BigEndian.Uint64(raw))
		nsec := int64(secs * float64(time.Second))
		return time.Unix(appleEpochOffset, 0).UTC().Add(time.Duration(nsec)), nil

	case 0x4: // data
		n, start, err := r.count(arg, body)
		if err != nil {
			return nil, err
		}
		raw, err := r.slice(start, n)
		if err != nil {
			return nil, err
		}
		out := make([]byte, len(raw))
		copy(out, raw)
		return out, nil

	case 0x5: // ASCII string
		n, start, err := r.count(arg, body)
		if err != nil {
			return nil, err
		}
		raw, err := r.slice(start, n)
		if err != nil {
			return nil, err
		}
		return string(raw), nil

	case 0x6: // UTF-16BE string, count is in code units
		n, start, err := r.count(arg, body)
		if err != nil {
			return nil, err
		}
		raw, err := r.slice(start, n*2)
		if err != nil {
			return nil, err
		}
		units := make([]uint16, n)
		for i := range units {
			units[i] = binary.BigEndian.Uint16(raw[i*2:])
		}
		return string(utf16.Decode(units)), nil

	case 0x8: // UID, arg+1 bytes; surfaced as an integer
		raw, err := r.slice(body, arg+1)
		if err != nil {
			return nil, err
		}
		return beUint(raw), nil

	case 0xa, 0xc: // array and set, both read as an ordered list
		n, start, err := r.count(arg, body)
		if err != nil {
			return nil, err
		}
		refs, err := r.refs(start, n)
		if err != nil {
			return nil, err
		}
		array := make([]interface{}, n)
		for i, ref := range refs {
			if array[i], err = r.object(ref); err != nil {
				return nil, err
			}
		}
		return array, nil

	case 0xd: // dict: n key refs followed by n value refs
		n, start, err := r.count(arg, body)
		if err != nil {
			return nil, err
		}
		keyRefs, err := r.refs(start, n)
		if err != nil {
			return nil, err
		}
		valueRefs, err := r.refs(start+n*uint64(r.objectRefSize), n)
		if err != nil {
			return nil, err
		}
		dict := make(map[string]interface{}, n)
		for i := range keyRefs {
			key, err := r.object(keyRefs[i])
			if err != nil {
				return nil, err
			}
			name, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("binary plist dictionary key is %T, not a string", key)
			}
			if dict[name], err = r.object(valueRefs[i]); err != nil {
				return nil, err
			}
		}
		return dict, nil
	}
	return nil, fmt.Errorf("unsupported binary plist marker 0x%02x", marker)
}

// count resolves the length of a sized object. An argument of 0xf means the
// length did not fit in the marker's low nibble and is stored as an integer
// object immediately after it.
func (r *bplistReader) count(arg, body uint64) (n, start uint64, err error) {
	if arg != 0xf {
		return arg, body, nil
	}
	if body >= uint64(len(r.data)) {
		return 0, 0, fmt.Errorf("binary plist object length is out of range")
	}
	marker := r.data[body]
	if marker>>4 != 0x1 {
		return 0, 0, fmt.Errorf("binary plist object length has marker 0x%02x", marker)
	}
	size := uint64(1) << (marker & 0x0f)
	raw, err := r.slice(body+1, size)
	if err != nil {
		return 0, 0, err
	}
	return beUint(raw), body + 1 + size, nil
}

func (r *bplistReader) refs(start, n uint64) ([]uint64, error) {
	width := uint64(r.objectRefSize)
	raw, err := r.slice(start, n*width)
	if err != nil {
		return nil, err
	}
	refs := make([]uint64, n)
	for i := range refs {
		refs[i] = beUint(raw[uint64(i)*width : (uint64(i)+1)*width])
	}
	return refs, nil
}

func (r *bplistReader) slice(off, n uint64) ([]byte, error) {
	if off > uint64(len(r.data)) || n > uint64(len(r.data))-off {
		return nil, fmt.Errorf("binary plist object extends past end of file")
	}
	return r.data[off : off+n], nil
}
