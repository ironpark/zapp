package dmg

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ironpark/zapp/pkg/macfs"
)

// Preserve unrelated resources and names when replacing or adding the icon.
func iconResourceFork(source macfs.Source, icon []byte) ([]byte, error) {
	if source == nil {
		return buildResourceFork(iconResourceType, iconResourceID, icon)
	}
	if source.Size() > 32<<20 {
		return nil, fmt.Errorf("existing resource fork is too large")
	}
	reader, err := source.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, 32<<20+1))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return buildResourceFork(iconResourceType, iconResourceID, icon)
	}
	bad := func() ([]byte, error) { return nil, fmt.Errorf("cannot safely update malformed resource fork") }
	if len(raw) < 16 {
		return bad()
	}
	u32 := func(b []byte) int { return int(binary.BigEndian.Uint32(b)) }
	u16 := func(b []byte) int { return int(binary.BigEndian.Uint16(b)) }
	d, m, dl, ml := u32(raw), u32(raw[4:]), u32(raw[8:]), u32(raw[12:])
	if d < 16 || m < 16 || ml < mapTypeListOffset || d > len(raw)-dl || m > len(raw)-ml {
		return bad()
	}
	data := append([]byte(nil), raw[d:d+dl]...)
	rm := raw[m : m+ml]
	tl, nl := u16(rm[24:]), u16(rm[26:])
	if tl < mapTypeListOffset || tl > len(rm)-2 || nl > len(rm) {
		return bad()
	}
	count := u16(rm[tl:]) + 1
	if count > 4096 || tl+2+count*typeEntrySize > len(rm) {
		return bad()
	}
	type resourceType struct {
		name string
		refs [][refEntrySize]byte
	}
	types := make([]resourceType, 0, count+1)
	found := false
	if len(data) > 0xffffff {
		return bad()
	}
	iconOffset := len(data)
	data = binary.BigEndian.AppendUint32(data, uint32(len(icon)))
	data = append(data, icon...)
	setIcon := func(ref *[refEntrySize]byte) {
		ref[5] = byte(iconOffset >> 16)
		ref[6] = byte(iconOffset >> 8)
		ref[7] = byte(iconOffset)
		clear(ref[8:])
	}
	for i := 0; i < count; i++ {
		entry := rm[tl+2+i*typeEntrySize:]
		n := u16(entry[4:]) + 1
		off := tl + u16(entry[6:])
		if n > 4096 || off < tl || off > len(rm)-n*refEntrySize {
			return bad()
		}
		typ := resourceType{name: string(entry[:4])}
		for j := 0; j < n; j++ {
			var ref [refEntrySize]byte
			copy(ref[:], rm[off+j*refEntrySize:off+(j+1)*refEntrySize])
			ro := int(ref[5])<<16 | int(ref[6])<<8 | int(ref[7])
			if ro > dl-4 || u32(raw[d+ro:]) > dl-ro-4 {
				return bad()
			}
			if typ.name == iconResourceType && int16(binary.BigEndian.Uint16(ref[:])) == iconResourceID {
				setIcon(&ref)
				found = true
			}
			typ.refs = append(typ.refs, ref)
		}
		types = append(types, typ)
	}
	if !found {
		var ref [refEntrySize]byte
		id := int16(iconResourceID)
		binary.BigEndian.PutUint16(ref[:], uint16(id))
		binary.BigEndian.PutUint16(ref[2:], 0xffff)
		setIcon(&ref)
		index := -1
		for i := range types {
			if types[i].name == iconResourceType {
				index = i
				break
			}
		}
		if index < 0 {
			types = append(types, resourceType{name: iconResourceType})
			index = len(types) - 1
		}
		types[index].refs = append(types[index].refs, ref)
	}
	names := rm[nl:]
	refsSize := 0
	for _, typ := range types {
		refsSize += len(typ.refs) * refEntrySize
	}
	nameOffset := mapTypeListOffset + 2 + len(types)*typeEntrySize + refsSize
	if nameOffset > 65535 {
		return nil, fmt.Errorf("resource map is too large")
	}
	newMap := make([]byte, nameOffset)
	copy(newMap[22:24], rm[22:24])
	binary.BigEndian.PutUint16(newMap[24:], mapTypeListOffset)
	binary.BigEndian.PutUint16(newMap[26:], uint16(nameOffset))
	binary.BigEndian.PutUint16(newMap[mapTypeListOffset:], uint16(len(types)-1))
	offset := 2 + len(types)*typeEntrySize
	for i, typ := range types {
		entry := newMap[mapTypeListOffset+2+i*typeEntrySize:]
		copy(entry, typ.name)
		binary.BigEndian.PutUint16(entry[4:], uint16(len(typ.refs)-1))
		binary.BigEndian.PutUint16(entry[6:], uint16(offset))
		for _, ref := range typ.refs {
			copy(newMap[mapTypeListOffset+offset:], ref[:])
			offset += refEntrySize
		}
	}
	newMap = append(newMap, names...)
	out := make([]byte, resourceHeaderSize)
	binary.BigEndian.PutUint32(out, resourceDataOffset)
	binary.BigEndian.PutUint32(out[4:], uint32(resourceDataOffset+len(data)))
	binary.BigEndian.PutUint32(out[8:], uint32(len(data)))
	binary.BigEndian.PutUint32(out[12:], uint32(len(newMap)))
	out = append(out, data...)
	out = append(out, newMap...)
	return out, nil
}
