package icns

import (
	"fmt"
	"image"
)

// Each channel has its own packet boundaries. Literal packets encode 1..128
// bytes; repeat packets encode 3..130 copies of a byte.
func decodeChannels(data []byte, img *image.NRGBA, channels []int) ([]byte, error) {
	n := len(img.Pix) / 4
	for _, ch := range channels {
		for i := 0; i < n; {
			if len(data) == 0 {
				return nil, fmt.Errorf("%w: truncated RLE packet", ErrFormat)
			}
			lead := int(data[0])
			data = data[1:]
			count := lead + 1
			if lead >= 128 {
				count = lead - 125
			}
			if count > n-i {
				return nil, fmt.Errorf("%w: RLE packet crosses channel boundary", ErrFormat)
			}
			if lead >= 128 {
				if len(data) == 0 {
					return nil, fmt.Errorf("%w: truncated RLE repeat", ErrFormat)
				}
				for j := 0; j < count; j++ {
					img.Pix[4*(i+j)+ch] = data[0]
				}
				data = data[1:]
			} else {
				if len(data) < count {
					return nil, fmt.Errorf("%w: truncated RLE literal", ErrFormat)
				}
				for j, v := range data[:count] {
					img.Pix[4*(i+j)+ch] = v
				}
				data = data[count:]
			}
			i += count
		}
	}
	return data, nil
}

func encodeRLE(src []byte) []byte {
	out := make([]byte, 0, len(src)+len(src)/128+1)
	for i := 0; i < len(src); {
		run := 1
		for run < 130 && i+run < len(src) && src[i+run] == src[i] {
			run++
		}
		if run >= 3 {
			out = append(out, byte(run+125), src[i])
			i += run
			continue
		}
		start := i
		for i < len(src) && i-start < 128 {
			if i+2 < len(src) && src[i] == src[i+1] && src[i] == src[i+2] {
				break
			}
			i++
		}
		out = append(out, byte(i-start-1))
		out = append(out, src[start:i]...)
	}
	return out
}
