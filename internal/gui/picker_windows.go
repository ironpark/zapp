package gui

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

const pickerScript = `[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
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
if ($env:ZAPP_PICK_FILTER) {$d.Filter=$env:ZAPP_PICK_FILTER}
if ($d.ShowDialog() -eq 'OK') { [Console]::Write($d.FileName) }
}`

func choosePath(ctx context.Context, mode pickMode, title, initial string) (string, error) {
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-Command", pickerScript)
	cmd.Env = append(os.Environ(), "ZAPP_PICK_MODE="+string(mode), "ZAPP_PICK_TITLE="+title, "ZAPP_PICK_INITIAL="+initial)
	if extensions := pickerExtensions(mode); len(extensions) > 0 && mode != pickApp {
		patterns := []string{}
		for _, ext := range extensions {
			patterns = append(patterns, "*."+ext)
		}
		cmd.Env = append(cmd.Env, "ZAPP_PICK_FILTER=Supported files|"+strings.Join(patterns, ";"))
	}
	// The dialog reports a dismissal as empty output, not an exit status.
	return runPicker(cmd, -1)
}
