package gui

import (
	"context"
	"os/exec"
	"path/filepath"
)

func choosePath(ctx context.Context, mode pickMode, title, initial string) (string, error) {
	args := []string{"--file-selection", "--title=" + title, "--filename=" + initial + string(filepath.Separator)}
	if mode == pickFolder || mode == pickApp {
		args = append(args, "--directory")
	}
	if mode == pickSave {
		args = append(args, "--save")
	}
	// zenity exits 1 when the dialog is dismissed.
	return runPicker(exec.CommandContext(ctx, "zenity", args...), 1)
}
