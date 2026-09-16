package comp

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
)

type Button struct {
	Bounds image.Rectangle
	Label  string
	Icon   Icon
	// IconOnly hides the label visually; retain Label for action descriptions.
	IconOnly           bool
	Selected, Disabled bool
	// Ghost removes the resting background and border; hover/selection remain visible.
	Ghost   bool
	Primary bool
	OnClick func()
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
		bg, fg = t.Selection, t.Text
	}
	if b.Primary {
		bg, fg = t.Accent, t.AccentText
		if pointer.In(b.Bounds) {
			bg = t.Text
		}
	}
	if b.Disabled {
		bg, fg = t.Disabled, t.DisabledText
	}
	if b.Ghost {
		fg = t.Text
		if b.Primary {
			fg = t.Accent
		}
		if b.Disabled {
			fg = t.DisabledText
		} else if b.Selected {
			RoundedRect(dst, b.Bounds, Radius, t.Selection)
		} else if pointer.In(b.Bounds) {
			RoundedRect(dst, b.Bounds, Radius, t.Hover)
		}
	} else {
		border := t.Border
		if b.Primary && !b.Disabled {
			border = bg
		}
		Surface(dst, b.Bounds, Radius, bg, border)
	}
	clip := b.Bounds.Intersect(dst.Bounds())
	if clip.Empty() {
		return
	}
	canvas := dst.SubImage(clip).(*ebiten.Image)
	iconWidth := 0
	if b.Icon != "" {
		iconWidth = 18
	}
	label := ""
	if !b.IconOnly || iconWidth == 0 {
		available := b.Bounds.Dx() - 24
		if iconWidth > 0 {
			available -= iconWidth + 8
		}
		label = p.Fit(b.Label, available, 14)
	}
	width := p.Measure(label, 14) + iconWidth
	if iconWidth > 0 && label != "" {
		width += 8
	}
	x := b.Bounds.Min.X + (b.Bounds.Dx()-width)/2
	if iconWidth > 0 {
		p.drawIcon(canvas, b.Icon, Box(x, b.Bounds.Min.Y+(b.Bounds.Dy()-18)/2, 18, 18), fg)
		x += iconWidth + 8
	}
	p.Text(canvas, label, x, p.TextY(b.Bounds, 14), 14, fg)
}
