package plist

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"time"
	"unicode/utf16"
)

// MarshalBinary renders a value as a binary property list ("bplist00"), the
// format Finder expects for the plist blobs stored inside a .DS_Store.
func MarshalBinary(value interface{}) ([]byte, error) {
	w := &bplistWriter{scalars: map[string]uint64{}}
	root, err := w.add(value)
	if err != nil {
		return nil, err
	}

	// Object references are only written once every object is known, so the
	// objects are encoded after the whole graph has been flattened.
	refSize := intSize(uint64(len(w.objects) - 1))
	body := &bytes.Buffer{}
	body.WriteString(bplistMagic)
	offsets := make([]uint64, len(w.objects))
	for i, obj := range w.objects {
		offsets[i] = uint64(body.Len())
		if err := w.encode(body, obj, refSize); err != nil {
			return nil, err
		}
	}

	tableOffset := uint64(body.Len())
	offsetSize := intSize(tableOffset)
	for _, off := range offsets {
		writeBEUint(body, off, offsetSize)
	}

	trailer := make([]byte, bplistTrailerLen)
	trailer[6] = offsetSize
	trailer[7] = refSize
	binary.BigEndian.PutUint64(trailer[8:16], uint64(len(w.objects)))
	binary.BigEndian.PutUint64(trailer[16:24], root)
	binary.BigEndian.PutUint64(trailer[24:32], tableOffset)
	body.Write(trailer)
	return body.Bytes(), nil
}

type bplistWriter struct {
	objects []interface{}
	// scalars maps an immutable value to the object that already encodes it,
	// so repeated keys and numbers are stored once.
	scalars map[string]uint64
}

// add flattens a value into the object table and returns its reference.
func (w *bplistWriter) add(value interface{}) (uint64, error) {
	switch v := value.(type) {
	case nil:
		return 0, fmt.Errorf("nil is not representable in a property list")
	case bool, string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		key := fmt.Sprintf("%T:%v", v, v)
		if ref, ok := w.scalars[key]; ok {
			return ref, nil
		}
		ref := w.append(v)
		w.scalars[key] = ref
		return ref, nil
	case []byte, time.Time:
		return w.append(v), nil
	case []interface{}:
		ref := w.append(v)
		refs := make([]uint64, len(v))
		for i, item := range v {
			child, err := w.add(item)
			if err != nil {
				return 0, err
			}
			refs[i] = child
		}
		w.objects[ref] = bplistArray{refs: refs}
		return ref, nil
	case map[string]interface{}:
		ref := w.append(v)
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		dict := bplistDict{keys: make([]uint64, len(keys)), values: make([]uint64, len(keys))}
		for i, k := range keys {
			keyRef, err := w.add(k)
			if err != nil {
				return 0, err
			}
			valueRef, err := w.add(v[k])
			if err != nil {
				return 0, err
			}
			dict.keys[i], dict.values[i] = keyRef, valueRef
		}
		w.objects[ref] = dict
		return ref, nil
	default:
		return 0, fmt.Errorf("unsupported plist value of type %T", value)
	}
}

func (w *bplistWriter) append(value interface{}) uint64 {
	w.objects = append(w.objects, value)
	return uint64(len(w.objects) - 1)
}

// bplistArray and bplistDict hold the resolved references of a container, which
// replace the original Go value in the object table once its children are added.
type bplistArray struct{ refs []uint64 }

type bplistDict struct{ keys, values []uint64 }

func (w *bplistWriter) encode(buf *bytes.Buffer, value interface{}, refSize uint8) error {
	switch v := value.(type) {
	case bool:
		if v {
			buf.WriteByte(0x09)
		} else {
			buf.WriteByte(0x08)
		}
	case int, int8, int16, int32, int64:
		writeBInt(buf, toInt64(v))
	case uint, uint8, uint16, uint32, uint64:
		writeBInt(buf, int64(toUint64(v)))
	case float32:
		buf.WriteByte(0x22)
		binary.Write(buf, binary.BigEndian, math.Float32bits(v))
	case float64:
		buf.WriteByte(0x23)
		binary.Write(buf, binary.BigEndian, math.Float64bits(v))
	case time.Time:
		buf.WriteByte(0x33)
		secs := float64(v.UTC().UnixNano())/float64(time.Second) - appleEpochOffset
		binary.Write(buf, binary.BigEndian, math.Float64bits(secs))
	case []byte:
		writeMarker(buf, 0x4, uint64(len(v)))
		buf.Write(v)
	case string:
		if isASCII(v) {
			writeMarker(buf, 0x5, uint64(len(v)))
			buf.WriteString(v)
			break
		}
		units := utf16.Encode([]rune(v))
		writeMarker(buf, 0x6, uint64(len(units)))
		for _, u := range units {
			binary.Write(buf, binary.BigEndian, u)
		}
	case bplistArray:
		writeMarker(buf, 0xa, uint64(len(v.refs)))
		for _, ref := range v.refs {
			writeBEUint(buf, ref, refSize)
		}
	case bplistDict:
		writeMarker(buf, 0xd, uint64(len(v.keys)))
		for _, ref := range v.keys {
			writeBEUint(buf, ref, refSize)
		}
		for _, ref := range v.values {
			writeBEUint(buf, ref, refSize)
		}
	default:
		return fmt.Errorf("unsupported plist value of type %T", value)
	}
	return nil
}

// writeMarker writes a type marker and its length, spilling the length into a
// following integer object when it does not fit in the marker's low nibble.
func writeMarker(buf *bytes.Buffer, kind byte, n uint64) {
	if n < 0x0f {
		buf.WriteByte(kind<<4 | byte(n))
		return
	}
	buf.WriteByte(kind<<4 | 0x0f)
	writeBInt(buf, int64(n))
}

func writeBInt(buf *bytes.Buffer, n int64) {
	switch {
	case n >= 0 && n <= math.MaxUint8:
		buf.WriteByte(0x10)
		buf.WriteByte(byte(n))
	case n >= 0 && n <= math.MaxUint16:
		buf.WriteByte(0x11)
		binary.Write(buf, binary.BigEndian, uint16(n))
	case n >= 0 && n <= math.MaxUint32:
		buf.WriteByte(0x12)
		binary.Write(buf, binary.BigEndian, uint32(n))
	default:
		// Negative values are always stored as 8 bytes, two's complement.
		buf.WriteByte(0x13)
		binary.Write(buf, binary.BigEndian, uint64(n))
	}
}

func writeBEUint(buf *bytes.Buffer, n uint64, size uint8) {
	for i := int(size) - 1; i >= 0; i-- {
		buf.WriteByte(byte(n >> (8 * uint(i))))
	}
}

// intSize reports the smallest power-of-two byte width that can hold n.
func intSize(n uint64) uint8 {
	switch {
	case n <= math.MaxUint8:
		return 1
	case n <= math.MaxUint16:
		return 2
	case n <= math.MaxUint32:
		return 4
	default:
		return 8
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int8:
		return int64(n)
	case int16:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	}
	return 0
}

func toUint64(v interface{}) uint64 {
	switch n := v.(type) {
	case uint:
		return uint64(n)
	case uint8:
		return uint64(n)
	case uint16:
		return uint64(n)
	case uint32:
		return uint64(n)
	case uint64:
		return n
	}
	return 0
}
