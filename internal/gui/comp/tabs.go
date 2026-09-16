package comp

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
	"unicode/utf8"
)

type Tab struct {
	Label    string
	Disabled bool
}
type Tabs struct {
	Bounds   image.Rectangle
	Items    []Tab
	Selected int
	Gap      int
	OnSelect func(int)
	Measure  func(string, int) int
}

// Buttons uses label widths plus padding for both rendering and hit testing.
// Measure should be the painter's font measurement function.
func (t Tabs) Buttons() []Button {
	measure := t.Measure
	if measure == nil {
		measure = func(s string, _ int) int { return utf8.RuneCountInString(s) * 8 }
	}
	x := t.Bounds.Min.X
	buttons := make([]Button, 0, len(t.Items))
	for i, item := range t.Items {
		if x >= t.Bounds.Max.X {
			break
		}
		width := min(max(64, measure(item.Label, 14)+36), t.Bounds.Max.X-x)
		bounds := Box(x, t.Bounds.Min.Y, width, t.Bounds.Dy())
		x += width + max(0, t.Gap)
		buttons = append(buttons, Button{Bounds: bounds, Label: item.Label, Selected: i == t.Selected, Disabled: item.Disabled, OnClick: func() {
			if t.OnSelect != nil {
				t.OnSelect(i)
			}
		}})
	}
	return buttons
}
func (t Tabs) Click(point image.Point) bool { return ClickButtons(t.Buttons(), point) }
func (t Tabs) Draw(dst *ebiten.Image, p *Painter, pointer image.Point) {
	buttons := t.Buttons()
	for _, b := range buttons {
		bg, fg := p.Theme.Background, p.Theme.Muted
		if pointer.In(b.Bounds) {
			bg = p.Theme.Hover
			fg = p.Theme.Text
		}
		if b.Selected {
			bg = p.Theme.Panel
			fg = p.Theme.Accent
		}
		if b.Disabled {
			fg = p.Theme.DisabledText
		}
		surface := b.Bounds
		if !b.Selected {
			surface.Max.Y--
		}
		Rect(dst, surface, bg)
		label := p.Fit(b.Label, b.Bounds.Dx()-20, 14)
		p.Text(dst, label, b.Bounds.Min.X+(b.Bounds.Dx()-p.Measure(label, 14))/2, b.Bounds.Min.Y+9, 14, fg)
		if b.Selected {
			// Draw only three edges: no bottom stroke needs painting over.
			Rect(dst, Box(b.Bounds.Min.X, b.Bounds.Min.Y, b.Bounds.Dx(), 1), p.Theme.Border)
			Rect(dst, Box(b.Bounds.Min.X, b.Bounds.Min.Y, 1, b.Bounds.Dy()), p.Theme.Border)
			Rect(dst, Box(b.Bounds.Max.X-1, b.Bounds.Min.Y, 1, b.Bounds.Dy()), p.Theme.Border)
		}
	}
	// Paint the baseline last, with exactly one opening for the active tab.
	left, right := t.Bounds.Min.X, t.Bounds.Max.X
	for _, b := range buttons {
		if b.Selected {
			Rect(dst, Box(left, t.Bounds.Max.Y-1, b.Bounds.Min.X-left+1, 1), p.Theme.Border)
			left = b.Bounds.Max.X - 1
			break
		}
	}
	Rect(dst, Box(left, t.Bounds.Max.Y-1, right-left, 1), p.Theme.Border)
}
