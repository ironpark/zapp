package icns

import (
	"bytes"
	"image"
	"testing"
)

func TestRLEPackets(t *testing.T) {
	for _, n := range []int{1, 2, 3, 127, 128, 129, 130, 131, 256, 1024} {
		for _, repeated := range []bool{false, true} {
			src := make([]byte, n)
			for i := range src {
				if repeated {
					src[i] = 42
				} else {
					src[i] = byte(i)
				}
			}
			img := image.NewNRGBA(image.Rect(0, 0, n, 1))
			rest, err := decodeChannels(encodeRLE(src), img, []int{0})
			if err != nil || len(rest) != 0 {
				t.Fatalf("n=%d: %v", n, err)
			}
			for i, v := range src {
				if img.Pix[4*i] != v {
					t.Fatal("pixel mismatch")
				}
			}
		}
	}
	for _, b := range [][]byte{{}, {0}, {127, 0}, {128}, {255, 0}} {
		if _, err := decodeChannels(b, image.NewNRGBA(image.Rect(0, 0, 4, 1)), []int{0}); err == nil {
			t.Fatalf("accepted %v", b)
		}
	}
	// Literal lead 0 and repeat leads 128/255 have distinct boundary values.
	if !bytes.Equal(encodeRLE([]byte{9, 9, 9}), []byte{128, 9}) {
		t.Fatal("three-byte run")
	}
	if !bytes.Equal(encodeRLE(bytes.Repeat([]byte{9}, 130)), []byte{255, 9}) {
		t.Fatal("130-byte run")
	}
}
func FuzzRLE(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 3, 3})
	f.Fuzz(func(t *testing.T, src []byte) {
		if len(src) > 1<<16 {
			return
		}
		img := image.NewNRGBA(image.Rect(0, 0, len(src), 1))
		rest, err := decodeChannels(encodeRLE(src), img, []int{0})
		if err != nil || len(rest) != 0 {
			t.Fatal(err)
		}
		for i, v := range src {
			if img.Pix[4*i] != v {
				t.Fatal("round trip")
			}
		}
	})
}
