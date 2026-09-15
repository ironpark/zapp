package macexec

import (
	"context"
	"os/exec"
)

// command runs the tool directly: on macOS it is already native.
func command(ctx context.Context, name string, args []string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// preflight has nothing to check: the tools are part of the system.
func preflight() error { return nil }
