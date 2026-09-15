// Package imageutil scales the icon images that go into an .icns or a DMG
// background.
package imageutil

import (
	"image"
	"image/draw"
	"math"
)

// lanczosRadius is the lobe count of the Lanczos window. Three is the usual
// choice for image scaling: sharp enough for icon artwork without the ringing
// that wider windows introduce.
const lanczosRadius = 3

// Resize scales img to width x height using a separable Lanczos3 filter.
//
// Filtering happens on alpha-premultiplied samples, which is what image.RGBA
// stores, so partially transparent icon edges do not bleed the color of fully
// transparent pixels.
func Resize(img image.Image, width, height int) *image.RGBA {
	if width <= 0 || height <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}
	src := toRGBA(img)
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw == 0 || sh == 0 {
		return image.NewRGBA(image.Rect(0, 0, width, height))
	}

	// Horizontal pass first, into a float buffer of the final width, then the
	// vertical pass; together these cost O(w*h*radius) instead of the O(w*h*r²)
	// of a single 2D convolution.
	horizontal := resample(rgbaToFloat(src), sw, sh, width, true)
	scaled := resample(horizontal, width, sh, height, false)

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for i, n := 0, width*height; i < n; i++ {
		a := clamp8(scaled[i*4+3])
		// Premultiplied color channels may not exceed alpha; filter overshoot
		// can push them past it, which would be an invalid RGBA value.
		dst.Pix[i*4+0] = clampTo(scaled[i*4+0], a)
		dst.Pix[i*4+1] = clampTo(scaled[i*4+1], a)
		dst.Pix[i*4+2] = clampTo(scaled[i*4+2], a)
		dst.Pix[i*4+3] = a
	}
	return dst
}

func toRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok && rgba.Bounds().Min == (image.Point{}) {
		return rgba
	}
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	return rgba
}

func rgbaToFloat(src *image.RGBA) []float64 {
	b := src.Bounds()
	out := make([]float64, b.Dx()*b.Dy()*4)
	for y := 0; y < b.Dy(); y++ {
		row := src.Pix[y*src.Stride : y*src.Stride+b.Dx()*4]
		copyTo := out[y*b.Dx()*4:]
		for i, v := range row {
			copyTo[i] = float64(v)
		}
	}
	return out
}

// resample scales one axis of an RGBA float image. When horizontal is true the
// image is w x h and is scaled to dstLen x h; otherwise it is scaled to
// w x dstLen.
func resample(pix []float64, w, h, dstLen int, horizontal bool) []float64 {
	srcLen := h
	if horizontal {
		srcLen = w
	}
	contribs := weights(srcLen, dstLen)

	dstW, dstH := w, h
	if horizontal {
		dstW = dstLen
	} else {
		dstH = dstLen
	}
	out := make([]float64, dstW*dstH*4)

	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			var sum [4]float64
			var c contribution
			if horizontal {
				c = contribs[x]
			} else {
				c = contribs[y]
			}
			for i, weight := range c.weights {
				srcX, srcY := x, y
				if horizontal {
					srcX = c.start + i
				} else {
					srcY = c.start + i
				}
				base := (srcY*w + srcX) * 4
				sum[0] += pix[base+0] * weight
				sum[1] += pix[base+1] * weight
				sum[2] += pix[base+2] * weight
				sum[3] += pix[base+3] * weight
			}
			base := (y*dstW + x) * 4
			copy(out[base:base+4], sum[:])
		}
	}
	return out
}

// contribution is the run of source samples that a single destination sample is
// built from, together with their normalized weights.
type contribution struct {
	start   int
	weights []float64
}

func weights(srcLen, dstLen int) []contribution {
	scale := float64(srcLen) / float64(dstLen)
	// Downscaling widens the filter so that every source pixel contributes,
	// which is what keeps a large icon from aliasing when it is minified.
	filterScale := math.Max(scale, 1)
	support := lanczosRadius * filterScale

	out := make([]contribution, dstLen)
	for i := range out {
		center := (float64(i)+0.5)*scale - 0.5
		start := int(math.Ceil(center - support))
		end := int(math.Floor(center + support))
		if start < 0 {
			start = 0
		}
		if end > srcLen-1 {
			end = srcLen - 1
		}
		w := make([]float64, 0, end-start+1)
		total := 0.0
		for j := start; j <= end; j++ {
			weight := lanczos((float64(j) - center) / filterScale)
			w = append(w, weight)
			total += weight
		}
		// Edge samples see a clipped filter; renormalizing keeps the image
		// from darkening along its borders.
		if total != 0 {
			for j := range w {
				w[j] /= total
			}
		}
		out[i] = contribution{start: start, weights: w}
	}
	return out
}

func lanczos(x float64) float64 {
	if x < 0 {
		x = -x
	}
	if x < 1e-9 {
		return 1
	}
	if x >= lanczosRadius {
		return 0
	}
	px := math.Pi * x
	return lanczosRadius * math.Sin(px) * math.Sin(px/lanczosRadius) / (px * px)
}

func clamp8(v float64) uint8 {
	return clampTo(v, 255)
}

func clampTo(v float64, max uint8) uint8 {
	n := int(math.Round(v))
	if n < 0 {
		return 0
	}
	if n > int(max) {
		return max
	}
	return uint8(n)
}
