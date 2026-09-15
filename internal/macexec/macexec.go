// Package macexec runs the macOS command line tools the mactools packages wrap.
//
// Every tool is invoked the same way: with a context so long-running work stays
// cancellable, with stdout and stderr captured separately so parsers never see
// diagnostics mixed into the data they parse, and with failures reported as an
// *Error carrying the full output for the caller to inspect.
package macexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Error reports a command that could not be started or exited non-zero.
type Error struct {
	Name   string // executable, as passed to Run
	Args   []string
	Stdout string
	Stderr string
	Err    error // underlying *exec.ExitError, context error, etc.
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("%s %s failed: %v", e.Name, strings.Join(e.Args, " "), e.Err)
	if out := e.Output(); out != "" {
		msg += fmt.Sprintf(" (output: %s)", out)
	}
	return msg
}

func (e *Error) Unwrap() error { return e.Err }

// Output returns stderr and stdout combined, trimmed, for display in messages
// and for matching against known diagnostics.
func (e *Error) Output() string {
	return strings.TrimSpace(strings.TrimSpace(e.Stderr) + "\n" + strings.TrimSpace(e.Stdout))
}

// Run executes name with args and returns its stdout. A non-zero exit or a
// cancelled context yields an *Error.
func Run(ctx context.Context, name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// A killed process reports "signal: killed" rather than the reason, so
		// prefer the context error when it is what stopped the command.
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
		}
		return stdout.String(), &Error{
			Name: name, Args: args,
			Stdout: stdout.String(), Stderr: stderr.String(),
			Err: err,
		}
	}
	return stdout.String(), nil
}

// Output returns the combined stdout and stderr of a failed command, or an
// empty string for any other error. It lets callers match on tool diagnostics
// without depending on how the error was wrapped.
func Output(err error) string {
	if cmdErr, ok := errors.AsType[*Error](err); ok {
		return cmdErr.Output()
	}
	return ""
}
