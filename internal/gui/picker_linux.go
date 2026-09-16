package gui

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

func choosePath(ctx context.Context, mode pickMode, title, initial string) (string, error) {
	args := []string{"--file-selection", "--title=" + title, "--filename=" + initial + string(filepath.Separator)}
	if mode == pickFolder || mode == pickApp {
		args = append(args, "--directory")
	}
	if mode == pickSave {
		args = append(args, "--save")
	}
	if extensions := pickerExtensions(mode); len(extensions) > 0 && mode != pickApp {
		patterns := []string{}
		for _, ext := range extensions {
			patterns = append(patterns, "*."+ext, "*."+strings.ToUpper(ext))
		}
		args = append(args, "--file-filter=Supported files | "+strings.Join(patterns, " "))
	}
	// zenity exits 1 when the dialog is dismissed.
	return runPicker(exec.CommandContext(ctx, "zenity", args...), 1)
}
