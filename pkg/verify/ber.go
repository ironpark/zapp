package verify

import (
	"errors"
)

// berToDER re-encodes the BER element b starts with as the DER
// encoding/asn1 reads: indefinite lengths become definite and constructed
// octet strings are joined. codesign writes its CMS signatures with
// indefinite lengths, and the space a signature is given may be padded.
func berToDER(b []byte) ([]byte, error) {
	out, _, err := convertBER(b, 0)
	return out, err
}

const maxBERDepth = 64

var errTruncated = errors.New("asn1: truncated element")

func convertBER(b []byte, depth int) (out, rest []byte, err error) {
	if depth > maxBERDepth {
		return nil, nil, errors.New("asn1: nested too deeply")
	}
	if len(b) < 2 {
		return nil, nil, errTruncated
	}
	i := 1
	if b[0]&0x1f == 0x1f {
		for i < len(b) && b[i]&0x80 != 0 {
			i++
		}
		i++
	}
	if i >= len(b) {
		return nil, nil, errTruncated
	}
	tag := b[:i]
	constructed := b[0]&0x20 != 0
	indefinite := b[i] == 0x80
	length := 0
	switch {
	case indefinite:
		if !constructed {
			return nil, nil, errors.New("asn1: indefinite length on a primitive element")
		}
		i++
	case b[i] < 0x80:
		length = int(b[i])
		i++
	default:
		n := int(b[i] & 0x7f)
		i++
		if n > 4 || i+n > len(b) {
			return nil, nil, errTruncated
		}
		for _, c := range b[i : i+n] {
			length = length<<8 | int(c)
		}
		i += n
	}
	if !constructed {
		if i+length > len(b) {
			return nil, nil, errTruncated
		}
		return encodeTLV(tag, b[i:i+length]), b[i+length:], nil
	}
	body := b[i:]
	if !indefinite {
		if length > len(body) {
			return nil, nil, errTruncated
		}
		body, rest = body[:length], body[length:]
	}
	var content []byte
	joinOctets := len(tag) == 1 && tag[0] == 0x24 // constructed OCTET STRING
	for {
		if indefinite {
			if len(body) < 2 {
				return nil, nil, errTruncated
			}
			if body[0] == 0 && body[1] == 0 {
				rest = body[2:]
				break
			}
		} else if len(body) == 0 {
			break
		}
		var child []byte
		if child, body, err = convertBER(body, depth+1); err != nil {
			return nil, nil, err
		}
		if joinOctets {
			child = child[contentOffset(child):]
		}
		content = append(content, child...)
	}
	if joinOctets {
		return encodeTLV([]byte{0x04}, content), rest, nil
	}
	return encodeTLV(tag, content), rest, nil
}

// contentOffset is where the content of a DER element starts.
func contentOffset(der []byte) int {
	i := 1
	if der[0]&0x1f == 0x1f {
		for der[i]&0x80 != 0 {
			i++
		}
		i++
	}
	if der[i] < 0x80 {
		return i + 1
	}
	return i + 1 + int(der[i]&0x7f)
}

func encodeTLV(tag, content []byte) []byte {
	out := append([]byte{}, tag...)
	n := len(content)
	switch {
	case n < 0x80:
		out = append(out, byte(n))
	default:
		var l []byte
		for ; n > 0; n >>= 8 {
			l = append([]byte{byte(n)}, l...)
		}
		out = append(out, 0x80|byte(len(l)))
		out = append(out, l...)
	}
	return append(out, content...)
}
