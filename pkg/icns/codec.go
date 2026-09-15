package icns

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

type encoding uint8

const (
	mono encoding = iota
	monoAlpha
	indexed4
	indexed8
	mask8
	rgb
	argb
	compressed
)

type format struct {
	w, h int
	kind encoding
	mask string
}

var formats = map[string]format{
	"ICON": {32, 32, mono, ""}, "ICN#": {32, 32, monoAlpha, ""},
	"icm#": {16, 12, monoAlpha, ""}, "icm4": {16, 12, indexed4, "icm#"}, "icm8": {16, 12, indexed8, "icm#"},
	"ics#": {16, 16, monoAlpha, ""}, "ics4": {16, 16, indexed4, "ics#"}, "ics8": {16, 16, indexed8, "ics#"},
	"icl4": {32, 32, indexed4, "ICN#"}, "icl8": {32, 32, indexed8, "ICN#"},
	"ich#": {48, 48, monoAlpha, ""}, "ich4": {48, 48, indexed4, "ich#"}, "ich8": {48, 48, indexed8, "ich#"},
	"is32": {16, 16, rgb, "s8mk"}, "il32": {32, 32, rgb, "l8mk"}, "ih32": {48, 48, rgb, "h8mk"}, "it32": {128, 128, rgb, "t8mk"},
	"s8mk": {16, 16, mask8, ""}, "l8mk": {32, 32, mask8, ""}, "h8mk": {48, 48, mask8, ""}, "t8mk": {128, 128, mask8, ""},
	"icp4": {16, 16, rgb, "s8mk"}, "icp5": {32, 32, rgb, "l8mk"}, "icp6": {64, 64, compressed, ""},
	"ic04": {16, 16, argb, ""}, "ic05": {32, 32, argb, ""},
	"ic07": {128, 128, compressed, ""}, "ic08": {256, 256, compressed, ""}, "ic09": {512, 512, compressed, ""}, "ic10": {1024, 1024, compressed, ""},
	"ic11": {32, 32, compressed, ""}, "ic12": {64, 64, compressed, ""}, "ic13": {256, 256, compressed, ""}, "ic14": {512, 512, compressed, ""},
	"icsb": {18, 18, argb, ""}, "icsB": {36, 36, compressed, ""}, "sb24": {24, 24, compressed, ""}, "SB24": {48, 48, compressed, ""},
}

func (f *ICNS) decodeImage(e Element) (image.Image, error) {
	s, ok := formats[e.Type]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupported, e.Type)
	}
	img, ownAlpha, err := decodePixels(e, s)
	if err != nil {
		return nil, fmt.Errorf("icns: %q: %w", e.Type, err)
	}
	if ownAlpha || s.mask == "" {
		return img, nil
	}
	for _, mask := range f.Elements {
		if mask.Type != s.mask {
			continue
		}
		n := s.w * s.h
		if formats[s.mask].kind == mask8 {
			if len(mask.Data) != n {
				return nil, fmt.Errorf("%w: %q mask length", ErrFormat, mask.Type)
			}
			for i, a := range mask.Data {
				img.Pix[4*i+3] = a
			}
		} else {
			if len(mask.Data) != n/4 {
				return nil, fmt.Errorf("%w: %q mask length", ErrFormat, mask.Type)
			}
			for i := range n {
				img.Pix[4*i+3] = 255 * ((mask.Data[n/8+i/8] >> uint(7-i%8)) & 1)
			}
		}
		break
	}
	return img, nil
}

func decodePixels(e Element, s format) (*image.NRGBA, bool, error) {
	data := e.Data
	// Modern slots can carry several encodings; inspect the signature first.
	if s.kind >= rgb && (bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) || isJPEG2000(data)) {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			if isJPEG2000(data) && err == image.ErrFormat {
				return nil, false, fmt.Errorf("%w: JPEG 2000 decoder is not registered", ErrUnsupported)
			}
			return nil, false, fmt.Errorf("%w: image header: %v", ErrFormat, err)
		}
		if cfg.Width != s.w || cfg.Height != s.h {
			return nil, false, fmt.Errorf("%w: image dimensions %dx%d, expected %dx%d", ErrFormat, cfg.Width, cfg.Height, s.w, s.h)
		}
		decoded, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, false, fmt.Errorf("%w: image data: %v", ErrFormat, err)
		}
		if decoded.Bounds().Dx() != s.w || decoded.Bounds().Dy() != s.h {
			return nil, false, ErrFormat
		}
		return pixels(decoded), true, nil
	}
	img := image.NewNRGBA(image.Rect(0, 0, s.w, s.h))
	n := s.w * s.h
	for i := range n {
		img.Pix[4*i+3] = 255
	}
	if s.kind >= rgb && bytes.HasPrefix(data, []byte("ARGB")) {
		rest, err := decodeChannels(data[4:], img, []int{3, 0, 1, 2})
		// A trailing NUL is Apple's ARM renderer workaround.
		if err == nil && len(rest) > 0 && (len(rest) != 1 || rest[0] != 0) {
			err = fmt.Errorf("%w: trailing ARGB data", ErrFormat)
		}
		return img, true, err
	}
	switch s.kind {
	case compressed, argb:
		return nil, false, ErrUnsupported
	case rgb:
		if e.Type == "it32" {
			if len(data) < 4 {
				return nil, false, ErrFormat
			}
			data = data[4:]
		}
		if len(data) == n*4 {
			for i := range n {
				copy(img.Pix[4*i:4*i+3], data[4*i+1:4*i+4])
			}
		} else {
			rest, err := decodeChannels(data, img, []int{0, 1, 2})
			if err != nil {
				return nil, false, err
			}
			if len(rest) != 0 {
				return nil, false, fmt.Errorf("%w: trailing RGB data", ErrFormat)
			}
		}
	case mono, monoAlpha:
		want := n / 8
		if s.kind == monoAlpha {
			want *= 2
		}
		if len(data) != want {
			return nil, false, fmt.Errorf("%w: mono length", ErrFormat)
		}
		for i := range n {
			v := 255 * (1 - ((data[i/8] >> uint(7-i%8)) & 1))
			img.Pix[4*i], img.Pix[4*i+1], img.Pix[4*i+2] = v, v, v
			if s.kind == monoAlpha {
				img.Pix[4*i+3] = 255 * ((data[n/8+i/8] >> uint(7-i%8)) & 1)
			}
		}
	case indexed4, indexed8:
		want := n
		palette := palette256
		if s.kind == indexed4 {
			want /= 2
			palette = palette16
		}
		if len(data) != want {
			return nil, false, fmt.Errorf("%w: indexed length", ErrFormat)
		}
		for i := range n {
			var index byte
			if s.kind == indexed4 {
				index = (data[i/2] >> uint(4*(1-i%2))) & 15
			} else {
				index = data[i]
			}
			img.SetNRGBA(i%s.w, i/s.w, palette[index].(color.NRGBA))
		}
	case mask8:
		if len(data) != n {
			return nil, false, fmt.Errorf("%w: mask length", ErrFormat)
		}
		for i, a := range data {
			img.Pix[4*i+3] = a
		}
	}
	return img, s.kind == monoAlpha || s.kind == mask8, nil
}

func isJPEG2000(b []byte) bool {
	return bytes.HasPrefix(b, []byte{0, 0, 0, 12, 'j', 'P', ' ', ' ', 13, 10, 135, 10}) || bytes.HasPrefix(b, []byte{255, 79, 255, 81})
}

func pixels(src image.Image) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.SetNRGBA(x, y, color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA))
		}
	}
	return dst
}

func encodeImage(typ string, src image.Image) ([]Element, error) {
	s, ok := formats[typ]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupported, typ)
	}
	if src == nil || src.Bounds().Dx() != s.w || src.Bounds().Dy() != s.h {
		return nil, fmt.Errorf("%w: wrong dimensions for %q", ErrFormat, typ)
	}
	img := pixels(src)
	n := s.w * s.h
	var data []byte
	switch s.kind {
	case compressed:
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
		data = buf.Bytes()
	case rgb, argb:
		channels := []int{0, 1, 2}
		if typ == "it32" {
			data = make([]byte, 4)
		}
		if s.kind == argb {
			data = []byte("ARGB")
			channels = []int{3, 0, 1, 2}
		}
		for _, ch := range channels {
			plane := make([]byte, n)
			for i := range plane {
				plane[i] = img.Pix[4*i+ch]
			}
			data = append(data, encodeRLE(plane)...)
		}
		if s.kind == argb {
			data = append(data, 0)
		}
	case mask8:
		data = make([]byte, n)
		for i := range data {
			data[i] = img.Pix[4*i+3]
		}
	case mono, monoAlpha:
		size := n / 8
		if s.kind == monoAlpha {
			size *= 2
		}
		data = make([]byte, size)
		for i := range n {
			c := img.NRGBAAt(i%s.w, i/s.w)
			// Convert straight RGB to luminance, independently of alpha.
			if (299*int(c.R) + 587*int(c.G) + 114*int(c.B)) < 128000 {
				data[i/8] |= 1 << uint(7-i%8)
			}
			if s.kind == monoAlpha && c.A >= 128 {
				data[n/8+i/8] |= 1 << uint(7-i%8)
			}
		}
	case indexed4, indexed8:
		size := n
		palette := palette256
		if s.kind == indexed4 {
			size /= 2
			palette = palette16
		}
		data = make([]byte, size)
		for i := range n {
			c := img.NRGBAAt(i%s.w, i/s.w)
			c.A = 255
			index := byte(palette.Index(c))
			if s.kind == indexed4 {
				data[i/2] |= index << uint(4*(1-i%2))
			} else {
				data[i] = index
			}
		}
	}
	result := []Element{{typ, data}}
	if s.mask != "" {
		mask, err := encodeImage(s.mask, src)
		if err != nil {
			return nil, err
		}
		result = append(result, mask...)
	}
	return result, nil
}
