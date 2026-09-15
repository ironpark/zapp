package icns

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"testing"
)

func loadFamily(t *testing.T, path string) *ICNS {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func samePixels(t *testing.T, got, want image.Image) {
	t.Helper()
	a, b := pixels(got), pixels(want)
	if a.Bounds() != b.Bounds() {
		t.Fatalf("bounds %v != %v", a.Bounds(), b.Bounds())
	}
	for i := 0; i < len(a.Pix); i += 4 {
		// PNG encoders may discard RGB values of completely transparent pixels.
		if a.Pix[i+3] == 0 && b.Pix[i+3] == 0 {
			continue
		}
		if !bytes.Equal(a.Pix[i:i+4], b.Pix[i:i+4]) {
			t.Fatalf("pixel %d: %v != %v", i/4, a.Pix[i:i+4], b.Pix[i:i+4])
		}
	}
}
func TestReferenceImages(t *testing.T) {
	cases := []struct{ file, typ, png string }{
		{"icon", "ICON", "icon"}, {"icm#", "icm#", "icm#"}, {"icm4", "icm4", "icm4"}, {"icm8", "icm8", "icm8"},
		{"is32", "is32", "16x16"}, {"il32", "il32", "32x32"}, {"it32", "it32", "128x128"},
		{"icp4", "icp4", "16x16"}, {"icp5", "icp5", "32x32"}, {"ic07", "ic07", "128x128"}, {"ic11", "ic11", "32x32"}, {"icsb", "icsb", "18x18"},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			f := loadFamily(t, "testdata/icns/"+c.file+".icns")
			got, err := f.Image(c.typ)
			if err != nil {
				t.Fatal(err)
			}
			file, err := os.Open("testdata/png/" + c.png + ".png")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			want, err := png.Decode(file)
			if err != nil {
				t.Fatal(err)
			}
			samePixels(t, got, want)
			// Re-encode in this slot and compare to the independent reference pixels.
			encoded := NewICNS()
			if err := encoded.AddWithType(c.typ, want); err != nil {
				t.Fatal(err)
			}
			round, err := encoded.Image(c.typ)
			if err != nil {
				t.Fatal(err)
			}
			samePixels(t, round, want)
		})
	}
}

func patterned(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(3, 5, 3+w, 5+h))
	for y := 5; y < 5+h; y++ {
		for x := 3; x < 3+w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{byte(x * 17), byte(y * 31), byte(x * y), byte(x + y)})
		}
	}
	return img
}
func TestAllFormats(t *testing.T) {
	for typ, s := range formats {
		t.Run(typ, func(t *testing.T) {
			img := patterned(s.w, s.h)
			f := NewICNS()
			if err := f.AddWithType(typ, img); err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := Encode(&buf, f); err != nil {
				t.Fatal(err)
			}
			decoded, err := Decode(&buf)
			if err != nil {
				t.Fatal(err)
			}
			got, err := decoded.Image(typ)
			if err != nil {
				t.Fatal(err)
			}
			if got.Bounds().Dx() != s.w || got.Bounds().Dy() != s.h {
				t.Fatal(got.Bounds())
			}
			if s.kind >= rgb {
				samePixels(t, got, img)
			}
		})
	}
}
func TestContainerRoundTrip(t *testing.T) {
	f := &ICNS{Elements: []Element{{"name", []byte("icon")}, {"????", []byte{1, 2, 3}}, {"????", nil}, {"TOC ", []byte("12345678")}}}
	var buf bytes.Buffer
	if err := Encode(&buf, f); err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(buf.Bytes())
	buf.WriteString("trailer")
	decoded, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "trailer" {
		t.Fatal("read beyond family")
	}
	var round bytes.Buffer
	if err := Encode(&round, decoded); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, round.Bytes()) {
		t.Fatal("elements changed")
	}
	if _, err := decoded.HighestResolution(); !errors.Is(err, ErrNoImage) {
		t.Fatal(err)
	}
	nested, err := (Element{Type: "sbtp", Data: original[8:]}).Nested()
	if err != nil {
		t.Fatal(err)
	}
	if len(nested.Elements) != len(f.Elements) {
		t.Fatal("nested elements lost")
	}
}
func TestMalformedContainers(t *testing.T) {
	packet := func(magic string, total uint32, typ string, size uint32, payload []byte) []byte {
		b := []byte(magic)
		b = binary.BigEndian.AppendUint32(b, total)
		if typ != "" {
			b = append(b, typ...)
			b = binary.BigEndian.AppendUint32(b, size)
			b = append(b, payload...)
		}
		return b
	}
	cases := [][]byte{
		nil, []byte("icns"), packet("nope", 8, "", 0, nil), packet("icns", 7, "", 0, nil), packet("icns", MaxFileSize+1, "", 0, nil),
		packet("icns", 9, "", 0, nil), packet("icns", 16, "is32", 0, nil), packet("icns", 16, "is32", 7, nil),
		packet("icns", 16, "is32", 9, nil), packet("icns", 17, "is32", 9, nil),
	}
	for i, b := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if _, err := Decode(bytes.NewReader(b)); err == nil {
				t.Fatal("accepted malformed input")
			}
		})
	}
}

type shortWriter struct{}

func (shortWriter) Write(b []byte) (int, error) { return len(b) / 2, nil }

type failingWriter struct{}

var errWriter = errors.New("write failure")

func (failingWriter) Write([]byte) (int, error) { return 0, errWriter }
func TestWriterErrors(t *testing.T) {
	if err := Encode(shortWriter{}, NewICNS()); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	if err := Encode(failingWriter{}, NewICNS()); !errors.Is(err, errWriter) {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	for _, f := range []*ICNS{nil, {Elements: []Element{{"bad", nil}}}} {
		if err := Encode(&buf, f); err == nil {
			t.Fatal("accepted invalid family")
		}
		if buf.Len() != 0 {
			t.Fatal("wrote before validation")
		}
	}
}
func TestSelectionAndReplacement(t *testing.T) {
	f := NewICNS()
	for _, size := range []int{16, 32, 64, 128, 256} {
		if err := f.Add(patterned(size, size)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := f.HighestResolution()
	if err != nil || got.Bounds().Dx() != 256 {
		t.Fatalf("%v %v", got, err)
	}
	got, err = f.ByResolution(32)
	if err != nil {
		t.Fatal(err)
	}
	samePixels(t, got, patterned(32, 32))
	if _, err = f.ByResolution(48); !errors.Is(err, ErrNoImage) {
		t.Fatal(err)
	}
	if _, err = f.ByResolution(0); !errors.Is(err, ErrNoImage) {
		t.Fatal(err)
	}
	before := len(f.Elements)
	f.Elements = append(f.Elements, Element{"TOC ", nil}, Element{"il32", nil})
	if err = f.Add(patterned(32, 32)); err != nil {
		t.Fatal(err)
	}
	if len(f.Elements) != before {
		t.Fatal("did not remove duplicate or stale TOC")
	}
	for _, img := range []image.Image{nil, patterned(17, 17), patterned(32, 16)} {
		if err = f.Add(img); err == nil {
			t.Fatal("accepted invalid size")
		}
	}
	if err = f.AddWithType("ic11", patterned(16, 16)); err == nil {
		t.Fatal("accepted wrong retina size")
	}
}
func TestMaskOrderingAndAlpha(t *testing.T) {
	f := NewICNS()
	src := patterned(16, 16)
	if err := f.Add(src); err != nil {
		t.Fatal(err)
	}
	f.Elements[0], f.Elements[1] = f.Elements[1], f.Elements[0]
	got, err := f.Image("is32")
	if err != nil {
		t.Fatal(err)
	}
	samePixels(t, got, src)
	f.Elements[0].Data = f.Elements[0].Data[:1]
	if _, err = f.Image("is32"); !errors.Is(err, ErrFormat) {
		t.Fatal(err)
	}
	// A PNG in a legacy slot must ignore even a malformed external mask.
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	f.Elements[1].Data = buf.Bytes()
	got, err = f.Image("is32")
	if err != nil {
		t.Fatal(err)
	}
	samePixels(t, got, src)
}
func TestUncompressedRGB(t *testing.T) {
	for _, typ := range []string{"is32", "it32"} {
		t.Run(typ, func(t *testing.T) {
			s := formats[typ]
			data := bytes.Repeat([]byte{7, 31, 63, 127}, s.w*s.h)
			if typ == "it32" {
				data = append([]byte{9, 8, 7, 6}, data...)
			}
			f := &ICNS{Elements: []Element{{typ, data}}}
			img, err := f.Image(typ)
			if err != nil {
				t.Fatal(err)
			}
			c := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
			if c != (color.NRGBA{31, 63, 127, 255}) {
				t.Fatal(c)
			}
		})
	}
}
func TestJPEG2000Preservation(t *testing.T) {
	f := loadFamily(t, "testdata/icns/ic12.icns")
	if _, err := f.HighestResolution(); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	if err := f.Add(patterned(16, 16)); err != nil {
		t.Fatal(err)
	}
	img, err := f.HighestResolution()
	if err != nil || img.Bounds().Dx() != 16 {
		t.Fatalf("fallback: %v", err)
	}
}
func TestMalformedImages(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, patterned(32, 32)); err != nil {
		t.Fatal(err)
	}
	for _, e := range []Element{{"icp4", buf.Bytes()}, {"ic04", []byte("ARGB")}, {"it32", []byte{0}}, {"ics4", nil}, {"ICN#", nil}, {"is32", []byte{255, 1}}} {
		t.Run(e.Type, func(t *testing.T) {
			f := &ICNS{Elements: []Element{e}}
			if _, err := f.Image(e.Type); !errors.Is(err, ErrFormat) {
				t.Fatal(err)
			}
		})
	}
}
func TestARGBARMWorkaround(t *testing.T) {
	// Independent example from relikd/icns-analysis: opaque blue, 16x16.
	data := []byte{'A', 'R', 'G', 'B', 255, 255, 251, 255, 255, 0, 251, 0, 255, 0, 251, 0, 255, 255, 251, 255}
	for _, padding := range []bool{false, true} {
		b := bytes.Clone(data)
		if padding {
			b = append(b, 0)
		}
		f := &ICNS{Elements: []Element{{"ic04", b}}}
		img, err := f.Image("ic04")
		if err != nil {
			t.Fatal(err)
		}
		if got := img.At(15, 15); got != (color.NRGBA{0, 0, 255, 255}) {
			t.Fatal(got)
		}
	}
}
func FuzzDecode(f *testing.F) {
	f.Add([]byte("icns\x00\x00\x00\x08"))
	seed, _ := os.ReadFile("testdata/icns/is32.icns")
	f.Add(seed)
	f.Fuzz(func(t *testing.T, b []byte) {
		icons, err := Decode(bytes.NewReader(b))
		if err != nil {
			return
		}
		_, _ = icons.HighestResolution()
		var buf bytes.Buffer
		if err := Encode(&buf, icons); err != nil {
			t.Fatal(err)
		}
		if _, err := Decode(&buf); err != nil {
			t.Fatal(err)
		}
	})
}
