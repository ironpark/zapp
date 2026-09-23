package comp

import (
	"slices"

	"github.com/ironpark/ggfx"
)

// Keyboard is a per-tick snapshot. Tests can supply it without starting a window.
type Keyboard struct {
	Command, Shift    bool
	Text              string
	Pressed, Repeated []ggfx.Key
}

func (k Keyboard) JustPressed(key ggfx.Key) bool {
	return slices.Contains(k.Pressed, key)
}
func (k Keyboard) Repeats(key ggfx.Key) bool {
	return k.JustPressed(key) || slices.Contains(k.Repeated, key)
}
