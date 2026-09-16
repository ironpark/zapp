package comp

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Keyboard is a per-tick snapshot. Tests can supply it without starting a window.
type Keyboard struct {
	Command, Shift    bool
	Text              string
	Pressed, Repeated []ebiten.Key
}

func (k Keyboard) JustPressed(key ebiten.Key) bool {
	for _, v := range k.Pressed {
		if v == key {
			return true
		}
	}
	return false
}
func (k Keyboard) Repeats(key ebiten.Key) bool {
	if k.JustPressed(key) {
		return true
	}
	for _, v := range k.Repeated {
		if v == key {
			return true
		}
	}
	return false
}
func CommandKey() bool {
	return ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)
}
func KeyRepeated(key ebiten.Key) bool {
	n := inpututil.KeyPressDuration(key)
	return inpututil.IsKeyJustPressed(key) || (n > 25 && n%3 == 0)
}
func CaptureKeyboard() Keyboard {
	k := Keyboard{Command: CommandKey(), Shift: ebiten.IsKeyPressed(ebiten.KeyShift), Text: string(ebiten.AppendInputChars(nil))}
	for _, key := range []ebiten.Key{ebiten.KeyEscape, ebiten.KeyTab, ebiten.KeyEnter, ebiten.KeySpace, ebiten.KeyA, ebiten.KeyC, ebiten.KeyX, ebiten.KeyV, ebiten.KeyArrowLeft, ebiten.KeyArrowRight, ebiten.KeyArrowUp, ebiten.KeyArrowDown, ebiten.KeyHome, ebiten.KeyEnd, ebiten.KeyBackspace, ebiten.KeyDelete} {
		if inpututil.IsKeyJustPressed(key) {
			k.Pressed = append(k.Pressed, key)
		} else if KeyRepeated(key) {
			k.Repeated = append(k.Repeated, key)
		}
	}
	return k
}
