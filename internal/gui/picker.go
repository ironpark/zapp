package gui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type pickMode string

const (
	pickFile     pickMode = "file"
	pickFolder   pickMode = "folder"
	pickApp      pickMode = "app"
	pickSave     pickMode = "save"
	pickImage    pickMode = "image"
	pickIcon     pickMode = "icon"
	pickItemIcon pickMode = "item-icon"
)

func pickerExtensions(mode pickMode) []string {
	switch mode {
	case pickApp:
		return []string{"app"}
	case pickImage:
		return []string{"png", "jpg", "jpeg"}
	case pickItemIcon:
		return []string{"icns", "png", "jpg", "jpeg"}
	case pickIcon:
		return []string{"icns", "png"}
	}
	return nil
}

type pickResult struct {
	index int
	path  string
	err   error
}

func pathField(label string, value *string, hint string, mode pickMode) field {
	f := stringField(label, value, hint)
	f.Browse = true
	f.picker = mode
	return f
}
func (g *editor) browse(index int) {
	if g.picking != nil || index < 0 || index >= len(g.fields) || !g.fields[index].Browse {
		return
	}
	// Allow the picker to replace an invalid draft of this same field.
	if g.active != index && !g.commit() {
		return
	}
	f := g.fields[index]
	initial := pickerDirectory(g.s.Path, f.Value, f.picker)
	title := f.pickerTitle
	if title == "" {
		title = "Choose " + f.Label
	}
	g.startPicker(index, f.picker, title, initial)
}

const addItemPicker = -1

func (g *editor) addFile() {
	if g.picking != nil || g.tab != tabDMG || !g.enabled() || !g.commit() {
		return
	}
	g.startPicker(addItemPicker, pickFile, "Add file to DMG", pickerDirectory(g.s.Path, "", pickFile))
}

func (g *editor) startPicker(index int, mode pickMode, title, initial string) {
	ctx := g.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	results := make(chan pickResult, 1)
	g.picking = results
	go func() {
		path, err := choosePath(ctx, mode, title, initial)
		results <- pickResult{index, path, err}
	}()
}

// System pickers need a real absolute directory. Empty values and unresolved
// expressions start beside the configuration, independent of the process cwd.
func pickerDirectory(config, value string, mode pickMode) string {
	base, err := filepath.Abs(filepath.Dir(config))
	if err != nil {
		base = filepath.Dir(config)
	}
	initial := base
	if value != "" && !strings.Contains(value, "${") {
		initial = value
		if !filepath.IsAbs(initial) {
			initial = filepath.Join(base, initial)
		}
		if mode == pickApp {
			initial = filepath.Dir(initial)
		}
	}
	for {
		if info, err := os.Stat(initial); err == nil && info.IsDir() {
			return initial
		}
		parent := filepath.Dir(initial)
		if parent == initial {
			return base
		}
		initial = parent
	}
}

func (g *editor) pollPicker() {
	select {
	case result := <-g.picking:
		g.picking = nil
		if result.err != nil {
			g.report(result.err, "")
			return
		}
		if result.path == "" {
			return
		}
		if result.index == addItemPicker {
			area := g.previewArea()
			point := area.Min.Add(image.Pt(area.Dx()/2, area.Dy()/2))
			path := g.assetPath(pickedPath(g.s.Path, result.path))
			count, err := g.addDroppedPaths([]string{path}, point)
			if err != nil {
				g.report(err, "")
				return
			}
			if count == 0 {
				g.report(nil, "This file is already in the layout.")
				return
			}
			g.revealItem()
			g.report(nil, "Added file. Drag its icon to arrange, or edit Item details.")
			return
		}
		path := pickedPath(g.s.Path, result.path)
		g.focus(result.index)
		g.input.SetText(path)
		g.commit()
	default:
	}
}

// macOS panels canonicalize directories such as /tmp to /private/tmp.
// Normalize only parent directories so selected symlink files keep their names.
func pickedPath(config, selected string) string {
	base := filepath.Dir(config)
	if realBase, err := filepath.EvalSymlinks(base); err == nil {
		if realParent, err := filepath.EvalSymlinks(filepath.Dir(selected)); err == nil {
			base = realBase
			selected = filepath.Join(realParent, filepath.Base(selected))
		}
	}
	if relative, err := filepath.Rel(base, selected); err == nil {
		return relative
	}
	return selected
}

// runPicker runs a helper process that prints the chosen path. cancelCode is
// the exit status the helper uses for a dismissed dialog, or -1 when it has no
// such convention. Arguments and environment carry user values; no path is
// interpolated into code.
func runPicker(cmd *exec.Cmd, cancelCode int) (string, error) {
	output, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if exit.ExitCode() == cancelCode {
				return "", nil
			}
			if detail := strings.TrimSpace(string(exit.Stderr)); detail != "" {
				return "", fmt.Errorf("could not open file picker: %s", detail)
			}
		}
		return "", fmt.Errorf("could not open file picker: %w; enter a path directly", err)
	}
	return strings.TrimRight(string(output), "\r\n"), nil
}
