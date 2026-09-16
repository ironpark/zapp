package gui

import (
	"context"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// NSWorkspace understands Asset Catalog icons and the system's bundle metadata.
// Called by a worker; AppKit work is dispatched to the Cocoa main queue.
func nativeAppIcon(ctx context.Context, path string) []byte {
	result := make(chan []byte, 1)
	pickerMain(func() {
		if ctx.Err() != nil {
			result <- nil
			return
		}
		pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
		defer pool.Send(objc.RegisterName("drain"))
		workspace := objc.ID(objc.GetClass("NSWorkspace")).Send(objc.RegisterName("sharedWorkspace"))
		icon := workspace.Send(objc.RegisterName("iconForFile:"), pickerString(path))
		tiff := icon.Send(objc.RegisterName("TIFFRepresentation"))
		rep := objc.ID(objc.GetClass("NSBitmapImageRep")).Send(objc.RegisterName("imageRepWithData:"), tiff)
		props := objc.ID(objc.GetClass("NSDictionary")).Send(objc.RegisterName("dictionary"))
		data := rep.Send(objc.RegisterName("representationUsingType:properties:"), uint64(4), props)
		length := objc.Send[uint64](data, objc.RegisterName("length"))
		if length == 0 || length > 16<<20 {
			result <- nil
			return
		}
		out := make([]byte, int(length))
		data.Send(objc.RegisterName("getBytes:length:"), unsafe.Pointer(&out[0]), length)
		result <- out
	})
	select {
	case data := <-result:
		return data
	case <-ctx.Done():
		return nil
	}
}
