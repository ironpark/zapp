package macos

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/ironpark/zapp/pkg/macho"
)

// codeBundles are the directory extensions that codesign signs as a bundle
// rather than as a file.
var codeBundles = map[string]bool{
	".app": true, ".appex": true, ".bundle": true, ".framework": true,
	".plugin": true, ".systemextension": true, ".xpc": true,
}

// nestedCode lists the code inside an app bundle that has to be signed before
// the bundle itself, in the order Apple prescribes in place of codesign
// --deep: from the inside out. Each group holds paths at one depth, deepest
// first, so no path in a group lies inside another and a group can go to one
// codesign call.
//
// Loose Mach-O files, such as dylibs and helper tools, are listed themselves.
// A nested bundle is listed when it holds code, and its main executable is
// left to be signed with it; a bundle of resources alone is sealed by its
// parent like any other resource.
func nestedCode(app string) ([][]string, error) {
	var bundles, files []string
	err := filepath.WalkDir(app, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case path == app:
		case d.IsDir():
			if codeBundles[extOf(path)] {
				bundles = append(bundles, path)
			}
		case d.Type().IsRegular():
			if ok, err := isMachO(path); err != nil || !ok {
				return err
			}
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	main := map[string]bool{mainExecutable(app): true}
	var code []string
	for _, b := range bundles {
		if slices.ContainsFunc(files, func(f string) bool { return within(f, b) }) {
			code = append(code, b)
			main[mainExecutable(b)] = true
		}
	}
	for _, f := range files {
		if !main[f] {
			code = append(code, f)
		}
	}

	depth := func(path string) int { return strings.Count(path, string(filepath.Separator)) }
	slices.SortStableFunc(code, func(a, b string) int { return depth(b) - depth(a) })
	var groups [][]string
	for i, path := range code {
		if i == 0 || depth(path) != depth(code[i-1]) {
			groups = append(groups, nil)
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], path)
	}
	return groups, nil
}

// within reports whether path lies inside dir.
func within(path, dir string) bool {
	return strings.HasPrefix(path, dir+string(filepath.Separator))
}

// mainExecutable is where a bundle keeps the executable codesign signs as part
// of the bundle. A framework's is its current version's binary; any other
// bundle names its own in Info.plist, and one that does not is assumed to
// follow the convention of naming it after the bundle.
func mainExecutable(bundle string) string {
	name := strings.TrimSuffix(filepath.Base(bundle), filepath.Ext(bundle))
	if extOf(bundle) == ".framework" {
		// Versions/Current is a symlink, which the walk does not follow, so
		// resolve it to the path the binary was found at.
		if v, err := os.Readlink(filepath.Join(bundle, "Versions", "Current")); err == nil {
			return filepath.Join(bundle, "Versions", v, name)
		}
		return filepath.Join(bundle, name)
	}
	if info, err := appbundle.Open(filepath.Join(bundle, "Contents", "Info.plist")); err == nil {
		if exe, err := info.BundleExecutable(); err == nil && exe != "" {
			name = exe
		}
	}
	return filepath.Join(bundle, "Contents", "MacOS", name)
}

// isMachO reports whether the file at path is a Mach-O binary, thin or
// universal.
func isMachO(path string) (bool, error) {
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
	return macho.IsHeader(header[:n]), nil
}
