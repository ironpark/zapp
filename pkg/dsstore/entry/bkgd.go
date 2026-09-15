package entry

// NewBackgroundEntry creates a new background entry.
//func NewBackgroundEntry(filename string, opts map[string]interface{}) (*EntryItem, error) {
//	blob := make([]byte, 12+4)
//	binary.BigEndian.PutUint32(blob[0:], uint32(len(blob)-4))
//	if opts["color"] != nil {
//		copy(blob[4:], "ClrB")
//		return nil, fmt.Errorf("not implemented")
//	} else if opts["pictureByteLength"] != nil {
//		copy(blob[4:], "PctB")
//		binary.BigEndian.PutUint32(blob[8:], opts["pictureByteLength"].(uint32))
//	} else {
//		copy(blob[4:], "DefB")
//	}
//	return NewEntry(filename, TypeBackground, "blob", blob), nil
//}
