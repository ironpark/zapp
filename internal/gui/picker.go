package gui

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	guiruntime "github.com/ironpark/ggui/runtime"
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

func pathField(label string, value *string, hint string, mode pickMode) field {
	f := stringField(label, value, hint)
	f.Browse = true
	f.picker = mode
	return f
}

func (g *editor) browse(index int) {
	if index < 0 || index >= len(g.fields) || !g.fields[index].Browse {
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
	if g.tab != tabDMG || !g.enabled() || !g.commit() {
		return
	}
	g.startPicker(addItemPicker, pickFile, "Add file to DMG", pickerDirectory(g.s.Path, "", pickFile))
}

// startPicker runs the native dialog, which blocks the UI thread until it
// closes, so no second picker or edit can start meanwhile.
func (g *editor) startPicker(index int, mode pickMode, title, initial string) {
	if g.desktop == nil || g.desktop.dialogs == nil {
		g.report(fmt.Errorf("file picker is unavailable; enter a path directly"), "")
		return
	}
	dialog := guiruntime.FileDialog{Title: title, Directory: initial}
	if extensions := pickerExtensions(mode); len(extensions) > 0 {
		dialog.Filters = []guiruntime.FileFilter{{Name: title, Extensions: extensions}}
	}
	var path string
	var err error
	switch mode {
	case pickFolder:
		path, err = g.desktop.dialogs.PickFolder(dialog)
	case pickSave:
		path, err = g.desktop.dialogs.SaveFile(dialog)
	default:
		path, err = g.desktop.dialogs.OpenFile(dialog)
	}
	if errors.Is(err, guiruntime.ErrCanceled) {
		path, err = "", nil
	}
	g.picked(index, path, err)
	g.invalidate()
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

// picked applies a picker's result: a blank path is a cancellation.
func (g *editor) picked(index int, path string, err error) {
	if err != nil {
		g.report(err, "")
		return
	}
	if path == "" {
		return
	}
	if index == addItemPicker {
		area := g.previewArea()
		point := area.Min.Add(image.Pt(area.Dx()/2, area.Dy()/2))
		count, err := g.addDroppedPaths([]string{g.assetPath(pickedPath(g.s.Path, path))}, point)
		if err != nil {
			g.report(err, "")
			return
		}
		if count == 0 {
			g.report(nil, "This file is already in the layout.")
			return
		}
		g.report(nil, "Added file. Drag its icon to arrange, or edit Item details.")
		return
	}
	g.focus(index)
	g.input.SetText(pickedPath(g.s.Path, path))
	g.commit()
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
