package comp

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
)

// Panel groups related content with a consistent inset and title hierarchy.
type Panel struct {
	Bounds             image.Rectangle
	Title, Description string
}

func (v Panel) Content() image.Rectangle {
	top := 48
	if v.Description != "" {
		top = 76
	}
	return image.Rect(v.Bounds.Min.X+Padding, v.Bounds.Min.Y+top, v.Bounds.Max.X-Padding, v.Bounds.Max.Y-Padding)
}
func (v Panel) Draw(dst *ebiten.Image, p *Painter) {
	Surface(dst, v.Bounds, Radius, p.Theme.Panel, p.Theme.Border)
	p.Text(dst, p.Fit(v.Title, v.Bounds.Dx()-32, 16), v.Bounds.Min.X+Padding, v.Bounds.Min.Y+14, 16, p.Theme.Text)
	if v.Description != "" {
		p.Text(dst, p.Fit(v.Description, v.Bounds.Dx()-32, 12), v.Bounds.Min.X+Padding, v.Bounds.Min.Y+42, 12, p.Theme.Muted)
	}
}

// Badge communicates a short state independently of actionable controls.
type Badge struct {
	Bounds    image.Rectangle
	Label     string
	Highlight bool
}

func (b Badge) Draw(dst *ebiten.Image, p *Painter) {
	fg := p.Theme.Muted
	if b.Highlight {
		fg = p.Theme.Accent
	}
	RoundedRect(dst, b.Bounds, Radius, p.Theme.Panel)
	p.Text(dst, p.Fit(b.Label, b.Bounds.Dx()-16, 12), b.Bounds.Min.X+8, b.Bounds.Min.Y+5, 12, fg)
}
