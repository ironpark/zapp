package comp

import "image"

// SplitRow distributes remainder pixels among cells so the last cell ends at
// bounds.Max.X. Gaps shrink to fit narrow rows rather than overflowing them.
func SplitRow(bounds image.Rectangle, count, gap int) []image.Rectangle {
	if count <= 0 || bounds.Empty() {
		return nil
	}
	gap = min(max(0, gap), bounds.Dx()/max(1, count-1))
	width := max(0, bounds.Dx()-gap*(count-1))
	cells := make([]image.Rectangle, count)
	for i := range cells {
		left := bounds.Min.X + width*i/count + gap*i
		right := bounds.Min.X + width*(i+1)/count + gap*i
		cells[i] = image.Rect(left, bounds.Min.Y, right, bounds.Max.Y)
	}
	return cells
}
