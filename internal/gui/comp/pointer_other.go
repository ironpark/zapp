//go:build !darwin

package comp

import "github.com/hajimehoshi/ebiten/v2"

func ObservePointer() func()                   { return func() {} }
func PointerPosition(_, _ int) (int, int)      { return ebiten.CursorPosition() }
func PointerPressPosition(_, _ int) (int, int) { return ebiten.CursorPosition() }
