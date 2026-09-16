package gui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type pickMode string

const (
	pickFile   pickMode = "file"
	pickFolder pickMode = "folder"
	pickApp    pickMode = "app"
	pickSave   pickMode = "save"
)

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
	g.startPicker(index, f.picker, "Choose "+f.Label, initial)
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

// Arguments/environment carry user values; no path is interpolated into code.
func choosePath(ctx context.Context, mode pickMode, title, initial string) (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		return chooseNativePath(ctx, mode, title, initial)
	case "linux":
		args := []string{"--file-selection", "--title=" + title, "--filename=" + initial + string(filepath.Separator)}
		if mode == pickFolder || mode == pickApp {
			args = append(args, "--directory")
		}
		if mode == pickSave {
			args = append(args, "--save")
		}
		cmd = exec.CommandContext(ctx, "zenity", args...)
	case "windows":
		script := `[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
Add-Type -AssemblyName System.Windows.Forms
if ($env:ZAPP_PICK_MODE -eq 'folder' -or $env:ZAPP_PICK_MODE -eq 'app') {
$d=New-Object System.Windows.Forms.FolderBrowserDialog
$d.Description=$env:ZAPP_PICK_TITLE
$d.SelectedPath=$env:ZAPP_PICK_INITIAL
if ($d.ShowDialog() -eq 'OK') { [Console]::Write($d.SelectedPath) }
} else {
if ($env:ZAPP_PICK_MODE -eq 'save') {$d=New-Object System.Windows.Forms.SaveFileDialog;$d.OverwritePrompt=$false} else {$d=New-Object System.Windows.Forms.OpenFileDialog}
$d.Title=$env:ZAPP_PICK_TITLE
$d.InitialDirectory=$env:ZAPP_PICK_INITIAL
if ($d.ShowDialog() -eq 'OK') { [Console]::Write($d.FileName) }
}`
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-Command", script)
		cmd.Env = append(os.Environ(), "ZAPP_PICK_MODE="+string(mode), "ZAPP_PICK_TITLE="+title, "ZAPP_PICK_INITIAL="+initial)
	default:
		return "", fmt.Errorf("file picker is unavailable on %s; enter a path directly", runtime.GOOS)
	}
	output, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if runtime.GOOS == "linux" && errors.As(err, &exit) && exit.ExitCode() == 1 {
			return "", nil
		}
		detail := ""
		if errors.As(err, &exit) {
			detail = strings.TrimSpace(string(exit.Stderr))
		}
		if detail != "" {
			return "", fmt.Errorf("could not open file picker: %s", detail)
		}
		return "", fmt.Errorf("could not open file picker: %w; enter a path directly", err)
	}
	return strings.TrimRight(string(output), "\r\n"), nil
}
