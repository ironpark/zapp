package gui

import (
	"context"
	"fmt"

	"github.com/ebitengine/purego/objc"
)

func pickerString(value string) objc.ID {
	return objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), value)
}
func pickerMain(fn func()) {
	block := objc.NewBlock(func(_ objc.Block) { fn() })
	defer block.Release()
	queue := objc.ID(objc.GetClass("NSOperationQueue")).Send(objc.RegisterName("mainQueue"))
	queue.Send(objc.RegisterName("addOperationWithBlock:"), block)
}

// AppKit panels belong to the editor window. All panel access happens on the
// Cocoa main queue; the Ebitengine update goroutine remains free to render.
func choosePath(ctx context.Context, mode pickMode, title, initial string) (string, error) {
	results := make(chan pickResult, 1)
	var panel objc.ID // accessed only on the main queue
	pickerMain(func() {
		if err := ctx.Err(); err != nil {
			results <- pickResult{err: err}
			return
		}
		pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
		defer pool.Send(objc.RegisterName("drain"))
		if mode == pickSave {
			panel = objc.ID(objc.GetClass("NSSavePanel")).Send(objc.RegisterName("savePanel"))
		} else {
			panel = objc.ID(objc.GetClass("NSOpenPanel")).Send(objc.RegisterName("openPanel"))
			panel.Send(objc.RegisterName("setCanChooseFiles:"), mode != pickFolder)
			panel.Send(objc.RegisterName("setCanChooseDirectories:"), mode == pickFolder)
			panel.Send(objc.RegisterName("setAllowsMultipleSelection:"), false)
			panel.Send(objc.RegisterName("setTreatsFilePackagesAsDirectories:"), false)
			if mode == pickApp {
				types := objc.ID(objc.GetClass("NSArray")).Send(objc.RegisterName("arrayWithObject:"), pickerString("app"))
				panel.Send(objc.RegisterName("setAllowedFileTypes:"), types)
			}
		}
		if panel == 0 {
			results <- pickResult{err: fmt.Errorf("could not create system file picker")}
			return
		}
		panel.Send(objc.RegisterName("setTitle:"), pickerString(title))
		panel.Send(objc.RegisterName("setMessage:"), pickerString(title))
		url := objc.ID(objc.GetClass("NSURL")).Send(objc.RegisterName("fileURLWithPath:"), pickerString(initial))
		panel.Send(objc.RegisterName("setDirectoryURL:"), url)
		app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
		window := app.Send(objc.RegisterName("keyWindow"))
		if window == 0 {
			window = app.Send(objc.RegisterName("mainWindow"))
		}
		completion := objc.NewBlock(func(_ objc.Block, response int64) {
			var path string
			if response == 1 {
				url := panel.Send(objc.RegisterName("URL"))
				str := url.Send(objc.RegisterName("path"))
				path = objc.Send[string](str, objc.RegisterName("UTF8String"))
			}
			panel = 0
			// Restore keyboard focus after the sheet has finished detaching.
			pickerMain(func() {
				if ctx.Err() == nil && window != 0 {
					app.Send(objc.RegisterName("activateIgnoringOtherApps:"), true)
					window.Send(objc.RegisterName("makeKeyAndOrderFront:"), objc.ID(0))
				}
			})
			results <- pickResult{path: path}
		})
		defer completion.Release()
		if window != 0 {
			panel.Send(objc.RegisterName("beginSheetModalForWindow:completionHandler:"), window, completion)
		} else {
			panel.Send(objc.RegisterName("beginWithCompletionHandler:"), completion)
		}
	})
	select {
	case result := <-results:
		return result.path, result.err
	case <-ctx.Done():
		pickerMain(func() {
			if panel != 0 {
				panel.Send(objc.RegisterName("cancel:"), objc.ID(0))
			}
		})
		return "", ctx.Err()
	}
}
