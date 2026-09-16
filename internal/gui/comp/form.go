package comp

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
)

// Form lays out a vertical list of inputs and owns its scroll offset. Field
// indexes are presentation-local; application bindings remain outside comp.
type Form struct {
	Bounds image.Rectangle
	Inputs []InputSpec
	offset int
}

func inputHeight(f InputSpec) int {
	if f.Multiline {
		return 165
	}
	return 89
}
func (f Form) Limit() int {
	height := 0
	for _, i := range f.Inputs {
		height += inputHeight(i)
	}
	return max(0, height-f.Bounds.Dy())
}
func (f Form) Offset() int                       { return f.offset }
func (f *Form) ScrollTo(offset int)              { f.offset = max(0, min(offset, f.Limit())) }
func (f *Form) ScrollBy(delta int)               { f.ScrollTo(f.offset + delta) }
func (f *Form) SetBounds(bounds image.Rectangle) { f.Bounds = bounds; f.ScrollTo(f.offset) }
func (f *Form) SetInputs(inputs []InputSpec)     { f.Inputs = inputs; f.ScrollTo(f.offset) }
func (f Form) FieldBounds(index int) image.Rectangle {
	if index < 0 || index >= len(f.Inputs) {
		return image.Rectangle{}
	}
	y := f.Bounds.Min.Y - f.offset
	for i := range index {
		y += inputHeight(f.Inputs[i])
	}
	height := 34
	if f.Inputs[index].Multiline {
		height = 108
	}
	return Box(f.Bounds.Min.X, y+25, f.Bounds.Dx(), height)
}
func (f Form) Hit(point image.Point) (int, bool) {
	if !point.In(f.Bounds) {
		return 0, false
	}
	for i := range f.Inputs {
		if point.In(f.FieldBounds(i)) {
			return i, true
		}
	}
	return 0, false
}
func (f *Form) Reveal(index int) {
	r := f.FieldBounds(index)
	if r.Empty() {
		return
	}
	offset := f.offset
	if r.Max.Y > f.Bounds.Max.Y {
		offset += r.Max.Y - f.Bounds.Max.Y
	}
	if r.Min.Y < f.Bounds.Min.Y+25 {
		offset -= f.Bounds.Min.Y + 25 - r.Min.Y
	}
	f.ScrollTo(offset)
}
func (f Form) Draw(dst *ebiten.Image, p *Painter, active int, draft *Input) {
	bounds := f.Bounds.Intersect(dst.Bounds())
	if bounds.Empty() {
		return
	}
	canvas := dst.SubImage(bounds).(*ebiten.Image)
	for i, spec := range f.Inputs {
		r := f.FieldBounds(i)
		row := image.Rect(r.Min.X, r.Min.Y-25, r.Max.X, r.Max.Y+19)
		if !row.Overlaps(bounds) {
			continue
		}
		if i == active && draft != nil {
			draft.Draw(canvas, p, r, true)
		} else {
			Input{Spec: spec}.Draw(canvas, p, r, false)
		}
	}
	if limit := f.Limit(); limit > 0 {
		track := Box(f.Bounds.Max.X+8, f.Bounds.Min.Y, 3, f.Bounds.Dy())
		Rect(dst, track, p.Theme.Border)
		height := min(track.Dy(), max(24, track.Dy()*track.Dy()/(limit+track.Dy())))
		y := track.Min.Y + (track.Dy()-height)*f.offset/limit
		Rect(dst, Box(track.Min.X, y, 3, height), p.Theme.Accent)
	}
}
