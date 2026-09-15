package macho

import (
	"fmt"
	"os"
	"strings"
)

// Info is what a Mach-O file records about its linkage.
type Info struct {
	// ID is the install name the file publishes, empty unless it is a dylib.
	ID string
	// Dependencies are the install names of the libraries it links against, in
	// load command order, with the file's own ID excluded.
	Dependencies []string
	// RPaths are its LC_RPATH entries, in load command order.
	RPaths []string
}

// Read reports the linkage of the Mach-O file at path. For a universal binary
// the architectures are merged: each architecture lists the same libraries, and
// a caller bundling them has no use for the distinction.
func Read(path string) (*Info, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return read(buf, path)
}

func read(buf []byte, path string) (*Info, error) {
	images, err := slices(buf)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	info := &Info{}
	seenDep := map[string]bool{}
	seenRPath := map[string]bool{}

	for _, s := range images {
		commands, err := s.commands(buf)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for _, c := range commands {
			switch {
			case c.cmd == cmdIDDylib:
				name, err := s.lcString(buf, c, 8)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", path, err)
				}
				// Every architecture of a dylib carries the same install name.
				if info.ID == "" {
					info.ID = name
				}
			case isDylibLoad(c.cmd):
				name, err := s.lcString(buf, c, 8)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", path, err)
				}
				if !seenDep[name] {
					seenDep[name] = true
					info.Dependencies = append(info.Dependencies, name)
				}
			case c.cmd == cmdRPath:
				p, err := s.lcString(buf, c, 8)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", path, err)
				}
				if !seenRPath[p] {
					seenRPath[p] = true
					info.RPaths = append(info.RPaths, p)
				}
			}
		}
	}

	// A dylib lists its own install name in LC_ID_DYLIB, never in a load
	// command, but a binary can legitimately link a library whose name matches;
	// otool -L prints the ID first, which callers then drop. Match that.
	if info.ID != "" {
		filtered := info.Dependencies[:0]
		for _, d := range info.Dependencies {
			if d != info.ID {
				filtered = append(filtered, d)
			}
		}
		info.Dependencies = filtered
	}
	return info, nil
}

// systemPrefixes are library locations provided by macOS itself. Libraries
// living there are always present on the target machine and must never be
// copied into an app bundle.
var systemPrefixes = []string{
	"/usr/lib/",
	"/System/",
	"/Library/Apple/",
}

// IsSystemLibrary reports whether an install name refers to a library shipped
// with macOS, which should be linked against in place rather than bundled.
func IsSystemLibrary(name string) bool {
	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
