package gui

import (
	"os/exec"
	"path/filepath"
	"runtime"
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
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	case "windows":
		cmd = exec.Command("explorer", "/select,", path)
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}
	if err := cmd.Start(); err != nil {
		g.report(err, "")
		return
	}
	go func() { _ = cmd.Wait() }()
}
