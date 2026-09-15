// Package icns reads and writes Apple icon families. It supports PNG, classic
// monochrome and indexed icons, and RGB/ARGB run-length encoded images.
// Unknown elements (including metadata, nested families and JPEG 2000 data)
// are preserved by Decode and Encode. JPEG 2000 image extraction requires a
// decoder registered with image.RegisterFormat.
package icns

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"io"
	"sort"
)

var (
	ErrFormat      = errors.New("icns: invalid format")
	ErrUnsupported = errors.New("icns: unsupported image encoding or type")
	ErrNoImage     = errors.New("icns: no image")
)

// MaxFileSize bounds memory use when reading or writing an icon family.
const MaxFileSize = 64 << 20
const maxElements = 4096

// Element contains a four-byte type code and its payload, without its header.
// Order, duplicate types and unknown payloads survive a Decode/Encode round trip.
type Element struct {
	Type string
	Data []byte
}

// ICNS is an ordered icon family. Its zero value is an empty family.
type ICNS struct{ Elements []Element }

// NewICNS creates an empty icon family.
func NewICNS() *ICNS { return &ICNS{} }

// Decode reads one family, leaving any following bytes in r unread. It checks
// container lengths; image payloads are decoded lazily by Image and the
// resolution selection methods.
func Decode(r io.Reader) (*ICNS, error) {
	var h [8]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return nil, fmt.Errorf("icns: header: %w", err)
	}
	n := int64(binary.BigEndian.Uint32(h[4:]))
	if string(h[:4]) != "icns" || n < 8 || n > MaxFileSize {
		return nil, fmt.Errorf("%w: file header or size", ErrFormat)
	}
	f := NewICNS()
	for remaining := n - 8; remaining > 0; {
		if remaining < 8 || len(f.Elements) >= maxElements {
			return nil, fmt.Errorf("%w: element header or count", ErrFormat)
		}
		if _, err := io.ReadFull(r, h[:]); err != nil {
			return nil, fmt.Errorf("icns: element header: %w", err)
		}
		size := int64(binary.BigEndian.Uint32(h[4:]))
		if size < 8 || size > remaining {
			return nil, fmt.Errorf("%w: element %q length %d", ErrFormat, h[:4], size)
		}
		// ReadAll grows with actual input, avoiding a large allocation for a
		// truncated element whose header advertises a large payload.
		data, err := io.ReadAll(io.LimitReader(r, size-8))
		if err != nil {
			return nil, fmt.Errorf("icns: element payload: %w", err)
		}
		if int64(len(data)) != size-8 {
			return nil, io.ErrUnexpectedEOF
		}
		f.Elements = append(f.Elements, Element{string(h[:4]), data})
		remaining -= size
	}
	return f, nil
}

// Encode writes a family without re-encoding its payloads. It validates all
// element sizes and type codes before writing anything to w.
func Encode(w io.Writer, f *ICNS) error {
	if f == nil || len(f.Elements) > maxElements {
		return fmt.Errorf("%w: nil family or too many elements", ErrFormat)
	}
	n := uint64(8)
	for _, e := range f.Elements {
		if len(e.Type) != 4 {
			return fmt.Errorf("%w: type must contain four bytes", ErrFormat)
		}
		n += 8 + uint64(len(e.Data))
		if n > MaxFileSize {
			return fmt.Errorf("%w: family exceeds size limit", ErrFormat)
		}
	}
	var h [8]byte
	copy(h[:4], "icns")
	binary.BigEndian.PutUint32(h[4:], uint32(n))
	if err := write(w, h[:]); err != nil {
		return err
	}
	for _, e := range f.Elements {
		copy(h[:4], e.Type)
		binary.BigEndian.PutUint32(h[4:], uint32(len(e.Data)+8))
		if err := write(w, h[:]); err != nil {
			return err
		}
		if err := write(w, e.Data); err != nil {
			return err
		}
	}
	return nil
}

func write(w io.Writer, b []byte) error {
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	return err
}

// Image extracts the first element of the requested type, combining its mask
// if necessary. A missing legacy mask is treated as fully opaque.
func (f *ICNS) Image(typ string) (image.Image, error) {
	for _, e := range f.Elements {
		if e.Type == typ {
			return f.decodeImage(e)
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrNoImage, typ)
}

// HighestResolution extracts the largest decodable image by pixel area.
// At equal sizes, full-color images are preferred to indexed/monochrome icons.
// Unsupported encodings are skipped; corrupt supported images return errors.
func (f *ICNS) HighestResolution() (image.Image, error) { return f.selectImage(0) }

// ByResolution extracts a square image with the given pixel side length.
func (f *ICNS) ByResolution(size int) (image.Image, error) {
	if size <= 0 {
		return nil, ErrNoImage
	}
	return f.selectImage(size)
}

func (f *ICNS) selectImage(size int) (image.Image, error) {
	var elements []Element
	for _, e := range f.Elements {
		s, ok := formats[e.Type]
		if ok && s.kind != mask8 && (size == 0 || s.w == size && s.h == size) {
			elements = append(elements, e)
		}
	}
	sort.SliceStable(elements, func(i, j int) bool {
		a, b := formats[elements[i].Type], formats[elements[j].Type]
		if a.w*a.h != b.w*b.h {
			return a.w*a.h > b.w*b.h
		}
		return a.kind > b.kind
	})
	var unsupported error
	for _, e := range elements {
		img, err := f.decodeImage(e)
		if errors.Is(err, ErrUnsupported) {
			unsupported = err
			continue
		}
		return img, err
	}
	if unsupported != nil {
		return nil, unsupported
	}
	return nil, ErrNoImage
}

// Add encodes an image at its native size, replacing elements of the chosen
// type and their mask. It uses RGB+mask for 16/32/48 pixels and PNG for larger
// sizes; 64 pixels uses ic12 (32@2x), avoiding the incompatible icp6 slot.
// Images are not resized. The encoded pixels are independent of the source.
func (f *ICNS) Add(img image.Image) error {
	if img == nil {
		return fmt.Errorf("%w: nil image", ErrFormat)
	}
	b := img.Bounds()
	typ := map[int]string{16: "is32", 18: "icsb", 24: "sb24", 32: "il32", 36: "icsB", 48: "ih32", 64: "ic12", 128: "ic07", 256: "ic08", 512: "ic09", 1024: "ic10"}[b.Dx()]
	if b.Dx() == 16 && b.Dy() == 12 {
		typ = "icm#"
	} else if b.Dx() != b.Dy() {
		typ = ""
	}
	if typ == "" {
		return fmt.Errorf("%w: image dimensions %dx%d", ErrUnsupported, b.Dx(), b.Dy())
	}
	return f.AddWithType(typ, img)
}

// AddWithType encodes an image using a specific four-byte type (including
// Retina types such as ic11). It creates any required mask automatically.
// Classic indexed images are quantized to their fixed palette.
func (f *ICNS) AddWithType(typ string, img image.Image) error {
	elements, err := encodeImage(typ, img)
	if err != nil {
		return err
	}
	for _, newElement := range elements {
		out := f.Elements[:0]
		for _, old := range f.Elements {
			if old.Type != newElement.Type && old.Type != "TOC " {
				out = append(out, old)
			}
		}
		f.Elements = append(out, newElement)
	}
	return nil
}

// Nested extracts a nested family payload (whose icns header is omitted).
func (e Element) Nested() (*ICNS, error) {
	if len(e.Data) > MaxFileSize-8 {
		return nil, ErrFormat
	}
	var h [8]byte
	copy(h[:4], "icns")
	binary.BigEndian.PutUint32(h[4:], uint32(len(e.Data)+8))
	return Decode(io.MultiReader(bytes.NewReader(h[:]), bytes.NewReader(e.Data)))
}
