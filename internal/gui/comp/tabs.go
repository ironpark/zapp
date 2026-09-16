package comp

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
	"unicode/utf8"
)

// TabStatus is the dot a tab shows beside its label. A typed enum keeps the
// producer and the draw loop from drifting on a spelling.
type TabStatus int

const (
	StatusNone TabStatus = iota
	StatusDisabled
	StatusEnabled
	StatusError
)

type Tab struct {
	Label    string
	Disabled bool
	Status   TabStatus
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
	for i, b := range t.Buttons() {
		fg := p.Theme.Muted
		if pointer.In(b.Bounds) && !b.Disabled {
			Rect(dst, b.Bounds.Inset(3), p.Theme.Hover)
			fg = p.Theme.Text
		}
		if b.Selected {
			fg = p.Theme.Text
			Rect(dst, Box(b.Bounds.Min.X+12, b.Bounds.Max.Y-3, b.Bounds.Dx()-24, 3), p.Theme.Accent)
		}
		if b.Disabled {
			fg = p.Theme.DisabledText
		}
		inset := 0
		if status := t.Items[i].Status; status != StatusNone {
			inset = 10
			c := p.Theme.DisabledText
			switch status {
			case StatusEnabled:
				c = p.Theme.Accent
			case StatusError:
				c = p.Theme.Error
			}
			RoundedRect(dst, Box(b.Bounds.Max.X-12, b.Bounds.Min.Y+(b.Bounds.Dy()-5)/2, 5, 5), 2, c)
		}
		label := p.Fit(b.Label, b.Bounds.Dx()-20-inset, 14)
		p.Text(dst, label, b.Bounds.Min.X+(b.Bounds.Dx()-inset-p.Measure(label, 14))/2, p.TextY(b.Bounds, 14), 14, fg)
	}
}
