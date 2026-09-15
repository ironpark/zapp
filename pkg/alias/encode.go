// alias/encode.go
package alias

import (
	"encoding/binary"
	"errors"
	"math"
	"time"
)

var AppleEpoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)

func AppleDate(value time.Time) uint32 {
	return uint32(math.Round(value.Sub(AppleEpoch).Seconds()))
}

type Extra struct {
	Type   int16
	Length uint16
	Data   []byte
}

type Info struct {
	Version int
	Target  struct {
		Type     string
		Filename string
		ID       uint32
		Created  time.Time
	}
	Volume struct {
		Name      string
		Created   time.Time
		Signature string
		Type      string
	}
	Parent struct {
		ID   uint32
		Name string
	}
	Extra []Extra
}

func Encode(info Info) ([]byte, error) {
	if info.Version != 2 {
		return nil, errors.New("unsupported version")
	}

	baseLength := 150
	extraLength := 0
	for _, e := range info.Extra {
		if int(e.Length) != len(e.Data) {
			return nil, errors.New("extra data length mismatch")
		}
		extraLength += 4 + int(e.Length)
		if e.Length%2 != 0 {
			extraLength++
		}
	}
	trailerLength := 4
	buf := make([]byte, baseLength+extraLength+trailerLength)

	binary.BigEndian.PutUint32(buf[0:], 0)
	binary.BigEndian.PutUint16(buf[4:], uint16(len(buf)))
	binary.BigEndian.PutUint16(buf[6:], uint16(info.Version))

	typeIndex := -1
	for i, t := range Type {
		if t == info.Target.Type {
			typeIndex = i
			break
		}
	}
	if typeIndex != 0 && typeIndex != 1 {
		return nil, errors.New("invalid target type")
	}
	binary.BigEndian.PutUint16(buf[8:], uint16(typeIndex))

	if len(info.Volume.Name) > 27 {
		return nil, errors.New("volume name too long")
	}
	buf[10] = byte(len(info.Volume.Name))
	copy(buf[11:38], make([]byte, 27))
	copy(buf[11:], []byte(info.Volume.Name))

	volCreateDate := AppleDate(info.Volume.Created)
	binary.BigEndian.PutUint32(buf[38:], volCreateDate)

	if info.Volume.Signature != "BD" && info.Volume.Signature != "H+" && info.Volume.Signature != "HX" {
		return nil, errors.New("invalid volume signature")
	}
	copy(buf[42:], []byte(info.Volume.Signature))

	volTypeIndex := -1
	for i, t := range VolumeType {
		if t == info.Volume.Type {
			volTypeIndex = i
			break
		}
	}
	if volTypeIndex < 0 || volTypeIndex > 5 {
		return nil, errors.New("invalid volume type")
	}
	binary.BigEndian.PutUint16(buf[44:], uint16(volTypeIndex))

	binary.BigEndian.PutUint32(buf[46:], info.Parent.ID)

	if len(info.Target.Filename) > 63 {
		return nil, errors.New("filename too long")
	}
	buf[50] = byte(len(info.Target.Filename))
	copy(buf[51:114], make([]byte, 63))
	copy(buf[51:], []byte(info.Target.Filename))

	binary.BigEndian.PutUint32(buf[114:], info.Target.ID)

	fileCreateDate := AppleDate(info.Target.Created)
	binary.BigEndian.PutUint32(buf[118:], fileCreateDate)

	copy(buf[122:], []byte("\x00\x00\x00\x00\x00\x00\x00\x00"))

	binary.BigEndian.PutUint16(buf[130:], 0xFFFF) // nlvlFrom
	binary.BigEndian.PutUint16(buf[132:], 0xFFFF) // nlvlTo

	binary.BigEndian.PutUint32(buf[134:], 0x00000D02) // volAttributes
	binary.BigEndian.PutUint16(buf[138:], 0x0000)     // volFSId

	copy(buf[140:150], make([]byte, 10)) // Reserved space

	pos := 150
	for _, e := range info.Extra {
		if e.Type < 0 {
			return nil, errors.New("invalid extra type")
		}
		binary.BigEndian.PutUint16(buf[pos:], uint16(e.Type))
		binary.BigEndian.PutUint16(buf[pos+2:], e.Length)
		copy(buf[pos+4:], e.Data)
		pos += 4 + int(e.Length)
		if e.Length%2 != 0 {
			buf[pos] = 0
			pos++
		}
	}

	binary.BigEndian.PutUint16(buf[pos:], 0xFFFF) // -1 in uint16
	binary.BigEndian.PutUint16(buf[pos+2:], 0)
	pos += 4

	if pos != len(buf) {
		return nil, errors.New("buffer length mismatch")
	}

	return buf, nil
}
