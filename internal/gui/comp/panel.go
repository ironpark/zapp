package comp

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// Panel groups related content with a consistent inset and title hierarchy.
type Panel struct {
	Bounds             image.Rectangle
	Title, Description string
	// TitleInset reserves space for controls aligned to the right of the title.
	TitleInset int
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
	p.Text(dst, p.Fit(v.Title, v.Bounds.Dx()-32-v.TitleInset, 16), v.Bounds.Min.X+Padding, p.TextY(Box(0, v.Bounds.Min.Y+8, 0, 32), 16), 16, p.Theme.Text)
	if v.Description != "" {
		p.Text(dst, p.Fit(v.Description, v.Bounds.Dx()-32, 12), v.Bounds.Min.X+Padding, v.Bounds.Min.Y+42, 12, p.Theme.Muted)
	}
}
