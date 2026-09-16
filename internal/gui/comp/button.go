package comp

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
)

type Button struct {
	Bounds             image.Rectangle
	Label              string
	Selected, Disabled bool
	OnClick            func()
}

// Click consumes a hit even when disabled, preventing click-through to content
// beneath a disabled control. Activation never depends on the displayed label.
func (b Button) Click(point image.Point) bool {
	if !point.In(b.Bounds) {
		return false
	}
	if !b.Disabled && b.OnClick != nil {
		b.OnClick()
	}
	return true
}
func ClickButtons(buttons []Button, point image.Point) bool {
	for _, b := range buttons {
		if b.Click(point) {
			return true
		}
	}
	return false
}
func (b Button) Draw(dst *ebiten.Image, p *Painter, pointer image.Point) {
	t := p.Theme
	bg, fg := t.Panel, t.Text
	if pointer.In(b.Bounds) {
		bg = t.Hover
	}
	if b.Selected {
		bg, fg = t.Accent, t.AccentText
	}
	if b.Disabled {
		bg, fg = t.Disabled, t.DisabledText
	}
	Surface(dst, b.Bounds, Radius, bg, t.Border)
	clip := b.Bounds.Intersect(dst.Bounds())
	if clip.Empty() {
		return
	}
	p.Text(dst.SubImage(clip).(*ebiten.Image), p.Fit(b.Label, b.Bounds.Dx()-24, 14), b.Bounds.Min.X+(b.Bounds.Dx()-p.Measure(p.Fit(b.Label, b.Bounds.Dx()-24, 14), 14))/2, b.Bounds.Min.Y+(b.Bounds.Dy()-20)/2, 14, fg)
}
