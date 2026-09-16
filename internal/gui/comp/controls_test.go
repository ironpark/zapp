package comp

import (
	"image"
	"testing"
)

func TestDisabledButtonConsumesWithoutActivation(t *testing.T) {
	called := false
	b := Button{Bounds: Box(10, 20, 100, 30), Label: "Any label", Disabled: true, OnClick: func() { called = true }}
	if !b.Click(image.Pt(11, 21)) || called {
		t.Fatal("disabled hit was not consumed safely")
	}
	b.Label = "Completely different"
	b.Disabled = false
	if !b.Click(image.Pt(11, 21)) || !called {
		t.Fatal("activation depends on label")
	}
	called = false
	if b.Click(image.Pt(110, 21)) || called {
		t.Fatal("right edge must be excluded")
	}
}

func TestTabsHitExactlyTheirDrawnBounds(t *testing.T) {
	selected := -1
	tabs := Tabs{Bounds: Box(17, 40, 317, 37), Items: []Tab{{Label: "One"}, {Label: "Two", Disabled: true}, {Label: "Three"}}, Gap: 7, OnSelect: func(i int) { selected = i }}
	buttons := tabs.Buttons()
	if buttons[2].Bounds.Max.X >= tabs.Bounds.Max.X || buttons[2].Bounds.Dx() <= buttons[0].Bounds.Dx() {
		t.Fatal("tabs must follow label widths, leaving unused space")
	}
	for i, b := range buttons {
		selected = -1
		if !tabs.Click(b.Bounds.Min.Add(image.Pt(1, 1))) {
			t.Fatal("missed drawn tab")
		}
		if i == 1 {
			if selected != -1 {
				t.Fatal("disabled tab activated")
			}
		} else if selected != i {
			t.Fatalf("selected %d, want %d", selected, i)
		}
	}
	if tabs.Click(image.Pt(buttons[0].Bounds.Max.X+1, 41)) {
		t.Fatal("tab gap consumed as a tab")
	}
}

func TestDialogBlocksBackgroundAndHandlesCancel(t *testing.T) {
	activated, cancelled := false, false
	d := Dialog{Visible: true, Bounds: Box(100, 100, 500, 190), Actions: []Button{{Label: "Confirm", OnClick: func() { activated = true }}}, OnCancel: func() { cancelled = true }}
	if !d.Handle(image.Pt(0, 0), true, false) || activated {
		t.Fatal("modal allowed click-through")
	}
	if !d.Handle(d.Buttons()[0].Bounds.Min.Add(image.Pt(1, 1)), true, false) || !activated {
		t.Fatal("dialog action not activated")
	}
	if !d.Handle(image.Point{}, false, true) || !cancelled {
		t.Fatal("dialog cancel not handled")
	}
	d.Visible = false
	if d.Handle(image.Point{}, true, false) {
		t.Fatal("hidden dialog consumed input")
	}
}

func TestFormScrollFocusAndResize(t *testing.T) {
	f := Form{Bounds: Box(20, 100, 300, 200), Inputs: []InputSpec{{Label: "One"}, {Label: "Two", Multiline: true}, {Label: "Three"}, {Label: "Four"}}}
	f.Reveal(3)
	last := f.FieldBounds(3)
	if !last.In(f.Bounds) || f.Offset() == 0 {
		t.Fatal("focused field was not revealed")
	}
	if index, ok := f.Hit(last.Min.Add(image.Pt(1, 1))); !ok || index != 3 {
		t.Fatal("scroll hit testing disagrees with layout")
	}
	if _, ok := f.Hit(image.Pt(21, 99)); ok {
		t.Fatal("clipped field accepted an outside click")
	}
	f.SetBounds(Box(20, 100, 300, 1000))
	if f.Offset() != 0 || f.Limit() != 0 {
		t.Fatal("resize did not clamp scroll")
	}
	f.ScrollBy(999)
	if f.Offset() != 0 {
		t.Fatal("short form scrolled")
	}
	f.SetBounds(Box(20, 100, 300, 200))
	f.ScrollTo(9999)
	if f.Offset() != f.Limit() {
		t.Fatal("overscroll was not clamped")
	}
	f.SetInputs(nil)
	if f.Offset() != 0 {
		t.Fatal("rebuild retained stale scroll")
	}
}

func TestSharedRowHitAndScroll(t *testing.T) {
	f := Form{Bounds: Box(20, 100, 317, 150), Inputs: []InputSpec{{Label: "Title"}, {Label: "Width"}, {Label: "Height", SameRow: true}, {Label: "Next"}}}
	f.Reveal(2)
	left, right := f.FieldBounds(1), f.FieldBounds(2)
	if left.Min.Y != right.Min.Y || left.Max.X+12 != right.Min.X || right.Max.X != f.Bounds.Max.X {
		t.Fatal("shared inputs are not aligned across the row")
	}
	for _, index := range []int{1, 2} {
		r := f.FieldBounds(index)
		if !r.In(f.Bounds) {
			t.Fatal("revealing a shared row clipped its inputs")
		}
		if got, ok := f.Hit(r.Min.Add(image.Pt(2, 2))); !ok || got != index {
			t.Fatal("shared input hit the wrong field")
		}
	}
	if _, ok := f.Hit(image.Pt(left.Max.X+2, left.Min.Y+2)); ok {
		t.Fatal("row gap accepted a hit")
	}
	if f.Limit() != 3*rowHeight-f.Bounds.Dy() {
		t.Fatal("shared row counted twice in scroll height")
	}
	f.Reveal(3)
	if !f.FieldBounds(3).In(f.Bounds) {
		t.Fatal("field after shared row could not be revealed")
	}
}

func TestBrowseAndInlineErrorGeometry(t *testing.T) {
	f := Form{Bounds: Box(20, 20, 332, 150), Inputs: []InputSpec{{Label: "Path", Browse: true}, {Label: "Size", Error: "Enter a valid size"}}}
	input, button := f.FieldBounds(0), f.BrowseBounds(0)
	if input.Overlaps(button) || button.Max.X != f.Bounds.Max.X {
		t.Fatal("browse button overlaps input")
	}
	if _, ok := f.Hit(button.Min.Add(image.Pt(2, 2))); ok {
		t.Fatal("browse click incorrectly focused text")
	}
	if index, ok := f.Hit(input.Min.Add(image.Pt(2, 2))); !ok || index != 0 {
		t.Fatal("path input not clickable")
	}
	f.Reveal(1)
	errorBottom := f.FieldBounds(1).Max.Y + 36
	if errorBottom > f.Bounds.Max.Y {
		t.Fatal("error remained clipped after revealing field")
	}
}
