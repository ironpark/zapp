package comp

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
)

// Segmented is a mutually exclusive group of toggle buttons.
type Segmented struct {
	Bounds   image.Rectangle
	Labels   []string
	Selected int
	OnSelect func(int)
}

func (s Segmented) Buttons() []Button {
	bounds := SplitRow(s.Bounds.Inset(3), len(s.Labels), 2)
	buttons := make([]Button, len(bounds))
	for i, r := range bounds {
		buttons[i] = Button{Bounds: r, Label: s.Labels[i], Selected: i == s.Selected, OnClick: func() {
			if i != s.Selected && s.OnSelect != nil {
				s.OnSelect(i)
			}
		}}
	}
	return buttons
}
func (s Segmented) Click(point image.Point) bool {
	if !point.In(s.Bounds) {
		return false
	}
	ClickButtons(s.Buttons(), point)
	return true
}
func (s Segmented) Draw(dst *ebiten.Image, p *Painter, pointer image.Point) {
	Surface(dst, s.Bounds, Radius, p.Theme.Input, p.Theme.Border)
	for _, b := range s.Buttons() {
		fg := p.Theme.Muted
		if b.Selected {
			RoundedRect(dst, b.Bounds, Radius-2, p.Theme.Selection)
			fg = p.Theme.Text
		} else if pointer.In(b.Bounds) {
			RoundedRect(dst, b.Bounds, Radius-2, p.Theme.Hover)
			fg = p.Theme.Text
		}
		label := p.Fit(b.Label, b.Bounds.Dx()-12, 12)
		p.Text(dst, label, b.Bounds.Min.X+(b.Bounds.Dx()-p.Measure(label, 12))/2, b.Bounds.Min.Y+(b.Bounds.Dy()-16)/2, 12, fg)
	}
}
