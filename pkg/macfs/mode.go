package macfs

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ironpark/zapp/pkg/macho"
)

// Mode is the mode the file at path, described by info, should have on a
// Mac. That is info's own, except on Windows: its file systems keep no
// execute bits, so every file there reads as 0666, and an app archived as
// is would not launch. There a file is made executable when a Mac would run
// it: anything in a bundle's MacOS directory, a Mach-O binary or a script.
func Mode(path string, info fs.FileInfo) (fs.FileMode, error) {
	if runtime.GOOS != "windows" {
		return info.Mode(), nil
	}
	return unixMode(path, info)
}

func unixMode(path string, info fs.FileInfo) (fs.FileMode, error) {
	mode := info.Mode()
	switch {
	case mode.IsDir():
		return fs.ModeDir | 0o755, nil
	case !mode.IsRegular():
		return mode, nil
	}
	perm := fs.FileMode(0o644)
	if mode&0o200 == 0 {
		perm = 0o444 // read-only
	}
	run, err := runnable(path)
	if err != nil {
		return 0, err
	}
	if run {
		perm |= 0o111
	}
	return perm, nil
}

func runnable(path string) (bool, error) {
	if filepath.Base(filepath.Dir(path)) == "MacOS" {
		return true, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	header := make([]byte, macho.HeaderSize)
	n, err := io.ReadFull(f, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return false, err
	}
	header = header[:n]
	return macho.IsHeader(header) || bytes.HasPrefix(header, []byte("#!")), nil
}
