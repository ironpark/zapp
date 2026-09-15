package macexec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// command runs the tool inside Darling, which supplies the macOS command line
// tools zapp drives. Paths are rewritten because the host filesystem is a
// separate volume inside the container.
func command(ctx context.Context, name string, args []string) *exec.Cmd {
	cwd, err := os.Getwd()
	if err != nil {
		// Without a working directory only absolute paths can resolve; let the
		// tool report what it could not find rather than failing here.
		cwd = "/"
	}
	argv := darlingArgv(cwd, name, args)
	return exec.CommandContext(ctx, argv[0], argv[1:]...)
}

// preflight reports a missing Darling installation as itself, rather than
// letting every tool fail with an unexplained "executable file not found".
func preflight() error {
	if _, err := exec.LookPath(darlingBin); err != nil {
		return fmt.Errorf("zapp drives the macOS command line tools, which on Linux come from Darling, "+
			"but %q is not on PATH: %w\nsee https://docs.darlinghq.org/installing-software.html", darlingBin, err)
	}
	return nil
}
