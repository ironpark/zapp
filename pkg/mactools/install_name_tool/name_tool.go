package install_name_tool

import (
	"fmt"
	"os/exec"
	"strings"
)

// Change rewrites a dependent shared library install name (LC_LOAD_DYLIB).
func Change(old string, new string, file string) error {
	return run("-change", old, new, file)
}

// ChangeId sets the install name of a shared library (LC_ID_DYLIB).
func ChangeId(new string, file string) error {
	return run("-id", new, file)
}

// AddRPath appends a runpath (LC_RPATH) entry.
func AddRPath(rpath string, file string) error {
	return run("-add_rpath", rpath, file)
}

func run(args ...string) error {
	cmd := exec.Command("install_name_tool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install_name_tool %s failed: %w (output: %s)",
			strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}
