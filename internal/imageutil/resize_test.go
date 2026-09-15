package imageutil

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestSizeAndBounds(t *testing.T) {
	out := Resize(solid(64, 32, color.RGBA{1, 2, 3, 255}), 17, 9)
	if got := out.Bounds(); got != image.Rect(0, 0, 17, 9) {
		t.Fatalf("bounds = %v", got)
	}
}

// A solid image must survive any scale unchanged: the filter weights sum to one
// everywhere, including the clipped windows at the edges.
func TestSolidColorIsPreserved(t *testing.T) {
	want := color.RGBA{10, 200, 90, 255}
	for _, size := range []int{1, 7, 64, 129, 512} {
		out := Resize(solid(97, 61, want), size, size)
		for y := range size {
			for x := range size {
				if got := out.RGBAAt(x, y); got != want {
					t.Fatalf("size %d, pixel (%d,%d) = %v, want %v", size, x, y, got, want)
				}
			}
		}
	}
}

func TestUpscaleKeepsCorners(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	src.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	src.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})
	src.SetRGBA(0, 1, color.RGBA{0, 0, 255, 255})
	src.SetRGBA(1, 1, color.RGBA{255, 255, 255, 255})

	out := Resize(src, 64, 64)
	checks := []struct {
		x, y int
		want color.RGBA
	}{
		{0, 0, color.RGBA{255, 0, 0, 255}},
		{63, 0, color.RGBA{0, 255, 0, 255}},
		{0, 63, color.RGBA{0, 0, 255, 255}},
		{63, 63, color.RGBA{255, 255, 255, 255}},
	}
	for _, c := range checks {
		got := out.RGBAAt(c.x, c.y)
		if !near(got, c.want, 12) {
			t.Errorf("pixel (%d,%d) = %v, want ~%v", c.x, c.y, got, c.want)
		}
	}
}

// Downscaling a checkerboard has to average, not point-sample: an aliasing
// resize would return one of the two source colors instead of their mean.
func TestDownscaleAverages(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := range 256 {
		for x := range 256 {
			if (x+y)%2 == 0 {
				src.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			} else {
				src.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	out := Resize(src, 16, 16)
	for y := range 16 {
		for x := range 16 {
			got := out.RGBAAt(x, y)
			if math.Abs(float64(got.R)-127.5) > 4 || got.A != 255 {
				t.Fatalf("pixel (%d,%d) = %v, want a mid gray", x, y, got)
			}
		}
	}
}

// Transparent regions must not tint the opaque ones, and color channels of a
// premultiplied pixel must never exceed its alpha.
func TestAlphaStaysPremultiplied(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := range 128 {
		for x := range 128 {
			if x < 64 {
				src.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			} else {
				src.SetRGBA(x, y, color.RGBA{0, 0, 0, 0})
			}
		}
	}
	out := Resize(src, 32, 32)
	for y := range 32 {
		for x := range 32 {
			p := out.RGBAAt(x, y)
			if p.R > p.A || p.G > p.A || p.B > p.A {
				t.Fatalf("pixel (%d,%d) = %v is not premultiplied", x, y, p)
			}
		}
		if p := out.RGBAAt(0, y); p.A != 255 {
			t.Fatalf("left edge pixel (0,%d) = %v, want opaque", y, p)
		}
		if p := out.RGBAAt(31, y); p.A != 0 {
			t.Fatalf("right edge pixel (31,%d) = %v, want transparent", y, p)
		}
	}
}

func TestNonRGBASourceAndOffsetBounds(t *testing.T) {
	src := image.NewNRGBA(image.Rect(10, 20, 42, 52))
	for y := 20; y < 52; y++ {
		for x := 10; x < 42; x++ {
			src.SetNRGBA(x, y, color.NRGBA{200, 100, 50, 255})
		}
	}
	out := Resize(src, 8, 8)
	if got := out.RGBAAt(4, 4); !near(got, color.RGBA{200, 100, 50, 255}, 1) {
		t.Fatalf("pixel = %v", got)
	}
}

func TestDegenerateSizes(t *testing.T) {
	if got := Resize(solid(4, 4, color.RGBA{}), 0, 10).Bounds(); !got.Empty() {
		t.Fatalf("bounds = %v, want empty", got)
	}
	if got := Resize(image.NewRGBA(image.Rect(0, 0, 0, 0)), 4, 4).Bounds(); got != image.Rect(0, 0, 4, 4) {
		t.Fatalf("bounds = %v", got)
	}
}

func near(a, b color.RGBA, tol int) bool {
	d := func(x, y uint8) int {
		n := int(x) - int(y)
		if n < 0 {
			return -n
		}
		return n
	}
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol && d(a.A, b.A) <= tol
}
