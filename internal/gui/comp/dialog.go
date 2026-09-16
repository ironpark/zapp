package comp

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
)

type Dialog struct {
	Visible        bool
	Bounds         image.Rectangle
	Title, Message string
	Actions        []Button
	OnCancel       func()
}

func (d Dialog) Buttons() []Button {
	positions := SplitRow(Box(d.Bounds.Min.X+24, d.Bounds.Max.Y-65, max(0, d.Bounds.Dx()-48), 36), len(d.Actions), 12)
	if len(positions) == 0 {
		return nil
	}
	buttons := make([]Button, len(d.Actions))
	for i, b := range d.Actions {
		b.Bounds = positions[i]
		buttons[i] = b
	}
	return buttons
}

// Handle consumes all input while visible, including clicks outside the dialog.
// Call this before background controls. The caller decides which keys cancel.
func (d Dialog) Handle(point image.Point, click, cancel bool) bool {
	if !d.Visible {
		return false
	}
	if cancel {
		if d.OnCancel != nil {
			d.OnCancel()
		}
		return true
	}
	if click {
		ClickButtons(d.Buttons(), point)
	}
	return true
}
func (d Dialog) Draw(dst *ebiten.Image, p *Painter, pointer image.Point) {
	if !d.Visible {
		return
	}
	t := p.Theme
	Rect(dst, dst.Bounds(), t.Overlay)
	Surface(dst, d.Bounds, Radius+3, t.Panel, t.Border)
	p.Text(dst, p.Fit(d.Title, d.Bounds.Dx()-48, 22), d.Bounds.Min.X+24, d.Bounds.Min.Y+22, 22, t.Text)
	p.Wrapped(dst, d.Message, d.Bounds.Min.X+24, d.Bounds.Min.Y+67, d.Bounds.Dx()-48, 15, t.Muted, 2)
	for _, b := range d.Buttons() {
		b.Draw(dst, p, pointer)
	}
}
