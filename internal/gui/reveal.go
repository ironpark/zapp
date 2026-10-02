package gui

import (
	"runtime"

	guiruntime "github.com/ironpark/ggui/runtime"
)

// revealLabel names the file manager the way the platform does.
func revealLabel() string {
	switch runtime.GOOS {
	case "darwin":
		return "Show in Finder"
	case "windows":
		return "Show in Explorer"
	}
	return "Open folder"
}

// reveal shows path in the platform's file manager, selected where the
// platform supports that. Failing to open one is reported, not fatal.
func (g *editor) reveal(path string) {
	if err := guiruntime.Reveal(path); err != nil {
		g.report(err, "")
	}
}
