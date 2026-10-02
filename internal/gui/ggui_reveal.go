package gui

import "github.com/ironpark/ggui"

// listReveal scrolls a row of a scrolled list into view without focusing
// it, which a FocusRef would. The DMG contents list needs this: focus stays
// on the canvas, whose arrow keys move the selected item.
//
// Each row records where it painted; every row paints, in view or not. A
// reveal asked for before its row first paints, as for a row just added,
// waits for that paint.
type listReveal struct {
	scroll  *ggui.ScrollWidget
	rows    map[string]ggui.Rect // where each row last painted
	pending string               // the row to reveal at its next paint
}

func newListReveal() *listReveal { return &listReveal{rows: map[string]ggui.Rect{}} }

// Scroll is the list's scroll view, bound to offset; Reveal moves it.
func (l *listReveal) Scroll(content ggui.Widget, offset ggui.Binding[float64]) ggui.Widget {
	l.scroll = ggui.Scroll(content).BindOffset(offset)
	clear(l.rows) // rows of the old content
	return l.scroll
}

// Row wraps the row with key, recording where it paints.
func (l *listReveal) Row(key string, row ggui.Widget) ggui.Widget {
	return ggui.FromFuncs(row.Layout, func(dst *ggui.Canvas, r ggui.Rect) {
		l.rows[key] = r
		if l.pending == key {
			l.pending = ""
			l.scroll.Reveal(r)
		}
		dst.Paint(row, r)
	})
}

// Reveal scrolls the row with key into view, moving as little as it must.
func (l *listReveal) Reveal(key string) {
	if r, ok := l.rows[key]; ok && l.scroll != nil {
		l.scroll.Reveal(r)
		return
	}
	l.pending = key
}
