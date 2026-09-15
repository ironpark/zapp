package macexec

import (
	"path/filepath"
	"strings"
)

// Darling runs macOS binaries on Linux. Inside its container the Linux
// filesystem is not the root: it is mounted as a separate volume, so every
// absolute host path has to be rewritten before a tool inside the container can
// see it.
//
// See https://docs.darlinghq.org/darling-shell.html
const (
	// darlingBin is the launcher on the host's PATH.
	darlingBin = "darling"
	// systemRoot is where Darling exposes the host filesystem.
	systemRoot = "/Volumes/SystemRoot"
)

// hostPath rewrites an absolute host path to the path the same file has inside
// the Darling container. Relative paths are left alone: they resolve against
// the working directory, which is translated separately.
func hostPath(path string) string {
	if !filepath.IsAbs(path) {
		return path
	}
	// Already translated, e.g. a path the caller read back out of a tool's
	// output. Rewriting it twice would break it.
	if path == systemRoot || strings.HasPrefix(path, systemRoot+"/") {
		return path
	}
	return filepath.Join(systemRoot, path)
}

// darlingArgv wraps a macOS tool invocation so it runs inside Darling, with the
// host's paths and working directory translated.
//
// The command goes through a shell so the working directory can be set: tools
// are given relative paths in places, and Darling would otherwise resolve them
// against the container's home directory rather than where zapp was run.
func darlingArgv(cwd, name string, args []string) []string {
	translated := make([]string, 0, len(args)+1)
	translated = append(translated, hostPath(name))
	for _, arg := range args {
		translated = append(translated, translateArg(arg))
	}

	var script strings.Builder
	script.WriteString("cd " + shellQuote(hostPath(cwd)) + " && exec")
	for _, tok := range translated {
		script.WriteString(" " + shellQuote(tok))
	}
	return []string{darlingBin, "shell", "/bin/sh", "-c", script.String()}
}

// translateArg rewrites the path in an argument. Tool arguments come as bare
// paths, and as --flag=/path, which is why the value is not simply the whole
// argument.
func translateArg(arg string) string {
	if strings.HasPrefix(arg, "/") {
		return hostPath(arg)
	}
	// --flag=/path and -flag=/path, but not a lone "-o" or a value like
	// "Developer ID Application: Someone (TEAMID)".
	if strings.HasPrefix(arg, "-") {
		if name, value, ok := strings.Cut(arg, "="); ok && strings.HasPrefix(value, "/") {
			return name + "=" + hostPath(value)
		}
	}
	return arg
}

// shellQuote renders s as a single shell word.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
