package otool

import (
	"fmt"
	"os/exec"
	"strings"
)

// systemPrefixes are library locations provided by macOS itself. Libraries
// living there are always present on the target machine and must never be
// copied into an app bundle.
var systemPrefixes = []string{
	"/usr/lib/",
	"/System/",
	"/Library/Apple/",
}

// IsSystemLib reports whether an install name refers to a library shipped with
// macOS, which should be linked against in place rather than bundled.
func IsSystemLib(name string) bool {
	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// GetDependencies returns the install names of the dynamic libraries file links
// against. The install name of file itself (LC_ID_DYLIB), which otool -L prints
// as the first entry for a dylib, is not included.
func GetDependencies(file string) ([]string, error) {
	output, err := run("-L", file)
	if err != nil {
		return nil, err
	}
	id, err := GetID(file)
	if err != nil {
		return nil, err
	}
	return parseOtoolOutput(output, id), nil
}

// GetID returns the install name (LC_ID_DYLIB) of file, or an empty string when
// file is not a dylib.
func GetID(file string) (string, error) {
	output, err := run("-D", file)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// otool prints a "<path>:" header per file and per architecture.
		if line == "" || strings.HasSuffix(line, ":") {
			continue
		}
		return line, nil
	}
	return "", nil
}

// GetRPaths returns the LC_RPATH entries of file, in load command order.
func GetRPaths(file string) ([]string, error) {
	output, err := run("-l", file)
	if err != nil {
		return nil, err
	}
	return parseRPaths(output), nil
}

func run(args ...string) (string, error) {
	cmd := exec.Command("otool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("otool %s failed: %w (output: %s)", strings.Join(args, " "), err, output)
	}
	return string(output), nil
}

func parseOtoolOutput(output, id string) []string {
	dependencies := make([]string, 0, 8)
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "(compatibility version") {
			continue
		}
		name := strings.Fields(line)[0]
		// A dylib lists its own install name first; a fat binary repeats the
		// whole list once per architecture.
		if name == id || seen[name] {
			continue
		}
		seen[name] = true
		dependencies = append(dependencies, name)
	}
	return dependencies
}

func parseRPaths(output string) []string {
	rpaths := make([]string, 0, 4)
	inRPath := false
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "cmd" && fields[0] != "path" {
			continue
		}
		if fields[0] == "cmd" {
			inRPath = fields[1] == "LC_RPATH"
			continue
		}
		if inRPath {
			rpaths = append(rpaths, fields[1])
			inRPath = false
		}
	}
	return rpaths
}
