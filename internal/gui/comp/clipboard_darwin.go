package comp

import (
	"fmt"
	"github.com/ebitengine/purego/objc"
)

func nativeString(s string) objc.ID {
	return objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), s)
}
func clipboardPool() objc.ID {
	return objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
}
func readClipboard() (string, error) {
	pool := clipboardPool()
	defer pool.Send(objc.RegisterName("drain"))
	board := objc.ID(objc.GetClass("NSPasteboard")).Send(objc.RegisterName("generalPasteboard"))
	value := board.Send(objc.RegisterName("stringForType:"), nativeString("public.utf8-plain-text"))
	if value == 0 {
		return "", nil
	}
	return objc.Send[string](value, objc.RegisterName("UTF8String")), nil
}
func writeClipboard(s string) error {
	pool := clipboardPool()
	defer pool.Send(objc.RegisterName("drain"))
	board := objc.ID(objc.GetClass("NSPasteboard")).Send(objc.RegisterName("generalPasteboard"))
	board.Send(objc.RegisterName("clearContents"))
	if !objc.Send[bool](board, objc.RegisterName("setString:forType:"), nativeString(s), nativeString("public.utf8-plain-text")) {
		return fmt.Errorf("could not write clipboard text")
	}
	return nil
}
