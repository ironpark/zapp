package comp

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
)

// Toggle requests a boolean change; the owner may reject it during validation.
type Toggle struct {
	Bounds            image.Rectangle
	Label             string
	Checked, Disabled bool
	OnChange          func(bool)
}

func (t Toggle) Click(point image.Point) bool {
	return (Button{Bounds: t.Bounds, Disabled: t.Disabled, OnClick: func() {
		if t.OnChange != nil {
			t.OnChange(!t.Checked)
		}
	}}).Click(point)
}
func (t Toggle) Draw(dst *ebiten.Image, p *Painter, pointer image.Point) {
	fg, track := p.Theme.Text, p.Theme.Border
	if t.Checked {
		track = p.Theme.Accent
	}
	if t.Disabled {
		fg, track = p.Theme.DisabledText, p.Theme.Disabled
	}
	if pointer.In(t.Bounds) && !t.Disabled {
		RoundedRect(dst, t.Bounds, Radius, p.Theme.Hover)
	}
	x, y := t.Bounds.Min.X+8, t.Bounds.Min.Y+(t.Bounds.Dy()-20)/2
	RoundedRect(dst, Box(x, y, 36, 20), 10, track)
	knob := x + 3
	if t.Checked {
		knob += 16
	}
	RoundedRect(dst, Box(knob, y+3, 14, 14), 7, p.Theme.Text)
	p.Text(dst, p.Fit(t.Label, t.Bounds.Dx()-60, 14), x+46, y, 14, fg)
}
