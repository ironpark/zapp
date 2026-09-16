package comp

import (
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/hajimehoshi/ebiten/v2"
)

// GLFW polls mouseLocationOutsideOfEventStream on macOS. That position can
// differ from the location of an event delivered by remote-control or assistive
// software. Use the coordinates actually delivered to our window instead.
// This monitor observes only this application's events and never consumes them.
var windowPointer struct {
	sync.Mutex
	x, y           float64 // fractions of the content view, independent of backing scale
	pressX, pressY float64
	valid          bool
}

type nativePoint struct{ X, Y float64 }
type nativeSize struct{ Width, Height float64 }
type nativeRect struct {
	Origin nativePoint
	Size   nativeSize
}

func ObservePointer() func() {
	windowPointer.Lock()
	windowPointer.valid = false
	windowPointer.Unlock()
	block := objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
		window := event.Send(objc.RegisterName("window"))
		if window == 0 {
			return event
		}
		view := window.Send(objc.RegisterName("contentView"))
		if view == 0 {
			return event
		}
		point := objc.Send[nativePoint](event, objc.RegisterName("locationInWindow"))
		point = objc.Send[nativePoint](view, objc.RegisterName("convertPoint:fromView:"), point, objc.ID(0))
		bounds := objc.Send[nativeRect](view, objc.RegisterName("bounds"))
		if bounds.Size.Width <= 0 || bounds.Size.Height <= 0 {
			return event
		}
		x, y := (point.X-bounds.Origin.X)/bounds.Size.Width, (point.Y-bounds.Origin.Y)/bounds.Size.Height
		if !objc.Send[bool](view, objc.RegisterName("isFlipped")) {
			y = 1 - y
		}
		windowPointer.Lock()
		windowPointer.x = x
		windowPointer.y = y
		if objc.Send[uint64](event, objc.RegisterName("type")) == 1 {
			windowPointer.pressX = x
			windowPointer.pressY = y
		}
		windowPointer.valid = true
		windowPointer.Unlock()
		return event
	})
	class := objc.ID(objc.GetClass("NSEvent"))
	// Left down/up, mouse moved/dragged and scroll-wheel events.
	mask := uint64(1<<1 | 1<<2 | 1<<5 | 1<<6 | 1<<22)
	monitor := class.Send(objc.RegisterName("addLocalMonitorForEventsMatchingMask:handler:"), mask, block)
	return func() {
		if monitor != 0 {
			class.Send(objc.RegisterName("removeMonitor:"), monitor)
		}
		block.Release()
		windowPointer.Lock()
		windowPointer.valid = false
		windowPointer.Unlock()
	}
}

func PointerPressPosition(w, h int) (int, int) {
	windowPointer.Lock()
	x, y, valid := windowPointer.pressX, windowPointer.pressY, windowPointer.valid
	windowPointer.Unlock()
	if valid {
		return int(x * float64(w)), int(y * float64(h))
	}
	return ebiten.CursorPosition()
}

func PointerPosition(w, h int) (int, int) {
	windowPointer.Lock()
	x, y, valid := windowPointer.x, windowPointer.y, windowPointer.valid
	windowPointer.Unlock()
	if valid {
		return int(x * float64(w)), int(y * float64(h))
	}
	return ebiten.CursorPosition()
}
