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

// Row geometry. Each row is a label, the editable box, then a hint; rowHeight
// covers all three plus the gap to the next row. Multiline rows are taller so a
// JSON value is legible without scrolling the box itself.
const (
	labelGutter    = 25
	hintGutter     = 19
	boxHeight      = 34
	multiBox       = 108
	rowHeight      = 89
	multiRowHeight = 165
)

// rowSpan is the vertical space one field occupies, including its gutters.
func rowSpan(f InputSpec) int {
	if f.Multiline {
		return multiRowHeight
	}
	return rowHeight
}

// boxSpan is the height of the editable area alone.
func boxSpan(f InputSpec) int {
	if f.Multiline {
		return multiBox
	}
	return boxHeight
}
func (f Form) Limit() int {
	height := 0
	for _, i := range f.Inputs {
		height += rowSpan(i)
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
		y += rowSpan(f.Inputs[i])
	}
	return Box(f.Bounds.Min.X, y+labelGutter, f.Bounds.Dx(), boxSpan(f.Inputs[index]))
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
	if r.Min.Y < f.Bounds.Min.Y+labelGutter {
		offset -= f.Bounds.Min.Y + labelGutter - r.Min.Y
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
		row := image.Rect(r.Min.X, r.Min.Y-labelGutter, r.Max.X, r.Max.Y+hintGutter)
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
