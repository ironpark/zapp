package comp

import (
	"image"
	"testing"
)

func TestFormGroupsAlignPairedInputsAndScroll(t *testing.T) {
	f := Form{Bounds: Box(10, 10, 400, 160), Inputs: []InputSpec{{Group: "Package", Label: "Type"}, {Group: "Component", Label: "ID"}, {Label: "Version", SameRow: true}, {Label: "Root"}}}
	a, b := f.FieldBounds(1), f.FieldBounds(2)
	if a.Min.Y != b.Min.Y || a.Overlaps(b) {
		t.Fatal("group misaligned paired inputs")
	}
	if f.Limit() != 3*rowHeight+2*groupGutter-f.Bounds.Dy() {
		t.Fatal("group spacing omitted from scroll range")
	}
	f.Reveal(2)
	if !f.FieldBounds(2).In(f.Bounds) {
		t.Fatal("paired input not revealed")
	}
	r := f.FieldBounds(1)
	if _, ok := f.Hit(image.Pt(r.Min.X+1, r.Min.Y-labelGutter-5)); ok {
		t.Fatal("group heading accepts input clicks")
	}
}
