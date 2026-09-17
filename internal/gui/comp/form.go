package comp

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
)

// Form lays out rows of inputs and owns its scroll offset. Field
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
	groupGutter    = 32
	labelGutter    = 25
	hintGutter     = 19
	boxHeight      = 32
	multiBox       = 108
	rowHeight      = 89
	multiRowHeight = 165
)

// rowSpan is the vertical space one field occupies, including its gutters.
func rowSpan(f InputSpec) int {
	height := rowHeight
	if f.Multiline {
		height = multiRowHeight
		if f.Height > 0 {
			height = f.Height + multiRowHeight - multiBox
		}
	}
	if f.Error != "" {
		height += 20
	}
	return height
}

// boxSpan is the height of the editable area alone.
func boxSpan(f InputSpec) int {
	if f.Multiline {
		return max(multiBox, f.Height)
	}
	return boxHeight
}

// row returns the exclusive end and height of a row.
func (f Form) row(start int) (end, height int) {
	end, height = start+1, rowSpan(f.Inputs[start])
	for end < len(f.Inputs) && f.Inputs[end].SameRow {
		height = max(height, rowSpan(f.Inputs[end]))
		end++
	}
	if f.Inputs[start].Group != "" {
		height += groupGutter
	}
	return
}

func (f Form) Limit() int {
	height := 0
	for i := 0; i < len(f.Inputs); {
		end, span := f.row(i)
		height += span
		i = end
	}
	return max(0, height-f.Bounds.Dy())
}
func (f Form) Offset() int                       { return f.offset }
func (f *Form) ScrollTo(offset int)              { f.offset = max(0, min(offset, f.Limit())) }
func (f *Form) ScrollBy(delta int)               { f.ScrollTo(f.offset + delta) }
func (f *Form) SetBounds(bounds image.Rectangle) { f.Bounds = bounds; f.ScrollTo(f.offset) }
func (f *Form) SetInputs(inputs []InputSpec)     { f.Inputs = inputs; f.ScrollTo(f.offset) }
func (f Form) cellBounds(index int) image.Rectangle {
	if index < 0 || index >= len(f.Inputs) {
		return image.Rectangle{}
	}
	y := f.Bounds.Min.Y - f.offset
	for start := 0; start < len(f.Inputs); {
		end, span := f.row(start)
		if index < end {
			if f.Inputs[start].Group != "" {
				y += groupGutter
			}
			row := Box(f.Bounds.Min.X, y+labelGutter, f.Bounds.Dx(), boxSpan(f.Inputs[index]))
			return SplitRow(row, end-start, 12)[index-start]
		}
		y += span
		start = end
	}
	return image.Rectangle{}
}
func (f Form) FieldBounds(index int) image.Rectangle {
	r := f.cellBounds(index)
	if !r.Empty() && f.Inputs[index].Browse {
		r.Max.X -= 90
	}
	return r
}
func (f Form) BrowseBounds(index int) image.Rectangle {
	r := f.cellBounds(index)
	if r.Empty() || !f.Inputs[index].Browse {
		return image.Rectangle{}
	}
	r.Min.X = r.Max.X - 82
	return r
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
	if f.Inputs[index].Error != "" {
		r.Max.Y += 36
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
func (f Form) Draw(dst *ebiten.Image, p *Painter, active int, draft *Input, pointer image.Point) {
	bounds := f.Bounds.Intersect(dst.Bounds())
	if bounds.Empty() {
		return
	}
	canvas := dst.SubImage(bounds).(*ebiten.Image)
	for i, spec := range f.Inputs {
		r := f.FieldBounds(i)
		row := image.Rect(r.Min.X, r.Min.Y-labelGutter, r.Max.X, r.Max.Y+hintGutter+20)
		if spec.Group != "" {
			row.Min.Y -= groupGutter
		}
		if !row.Overlaps(bounds) {
			continue
		}
		if spec.Group != "" {
			p.Text(canvas, p.Fit(spec.Group, f.Bounds.Dx(), 12), f.Bounds.Min.X, row.Min.Y+3, 12, p.Theme.Accent)
			Rect(canvas, Box(f.Bounds.Min.X, row.Min.Y+24, f.Bounds.Dx(), 1), p.Theme.Border)
		}
		if spec.Browse {
			(Button{Bounds: f.BrowseBounds(i), Label: "Browse"}).Draw(canvas, p, pointer)
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
		Scrollbar(dst, track, f.offset, limit, p.Theme.Accent)
	}
}
