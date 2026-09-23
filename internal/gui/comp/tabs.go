package comp

import (
	"github.com/ironpark/ggfx"
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
// layout yields each visible tab's index and bounds. Drawing needs only these,
// so it does not pay for a Button and a closure per tab on every frame.
func (t Tabs) layout(yield func(int, image.Rectangle)) {
	measure := t.Measure
	if measure == nil {
		measure = func(s string, _ int) int { return utf8.RuneCountInString(s) * 8 }
	}
	x := t.Bounds.Min.X
	for i, item := range t.Items {
		if x >= t.Bounds.Max.X {
			return
		}
		width := min(max(64, measure(item.Label, 14)+36), t.Bounds.Max.X-x)
		yield(i, Box(x, t.Bounds.Min.Y, width, t.Bounds.Dy()))
		x += width + max(0, t.Gap)
	}
}
func (t Tabs) Buttons() []Button {
	buttons := make([]Button, 0, len(t.Items))
	t.layout(func(i int, bounds image.Rectangle) {
		buttons = append(buttons, Button{Bounds: bounds, Label: t.Items[i].Label, Selected: i == t.Selected, Disabled: t.Items[i].Disabled, OnClick: func() {
			if t.OnSelect != nil {
				t.OnSelect(i)
			}
		}})
	})
	return buttons
}
func (t Tabs) Click(point image.Point) bool { return ClickButtons(t.Buttons(), point) }
func (t Tabs) Draw(dst *ggfx.Image, p *Painter, pointer image.Point) {
	t.layout(func(i int, bounds image.Rectangle) {
		fg := p.Theme.Muted
		if pointer.In(bounds) && !t.Items[i].Disabled {
			Rect(dst, bounds.Inset(3), p.Theme.Hover)
			fg = p.Theme.Text
		}
		if i == t.Selected {
			fg = p.Theme.Text
			Rect(dst, Box(bounds.Min.X+12, bounds.Max.Y-3, bounds.Dx()-24, 3), p.Theme.Accent)
		}
		if t.Items[i].Disabled {
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
			RoundedRect(dst, Box(bounds.Max.X-12, bounds.Min.Y+(bounds.Dy()-5)/2, 5, 5), 2, c)
		}
		label := p.Fit(t.Items[i].Label, bounds.Dx()-20-inset, 14)
		p.Text(dst, label, bounds.Min.X+(bounds.Dx()-inset-p.Measure(label, 14))/2, p.TextY(bounds, 14), 14, fg)
	})
}
