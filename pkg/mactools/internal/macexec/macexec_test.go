package macexec

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRunCapturesStdoutOnly(t *testing.T) {
	out, err := Run(context.Background(), "sh", "-c", "echo data; echo noise >&2")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	// Parsers must not see stderr mixed into their input.
	if got := strings.TrimSpace(out); got != "data" {
		t.Errorf("stdout = %q, want %q", got, "data")
	}
}

func TestRunFailureCarriesOutput(t *testing.T) {
	_, err := Run(context.Background(), "sh", "-c", "echo out; echo problem >&2; exit 3")
	if err == nil {
		t.Fatal("Run() on a failing command returned no error")
	}

	var cmdErr *Error
	if !errors.As(err, &cmdErr) {
		t.Fatalf("error is %T, want *macexec.Error", err)
	}
	if cmdErr.Name != "sh" {
		t.Errorf("Name = %q, want %q", cmdErr.Name, "sh")
	}
	if strings.TrimSpace(cmdErr.Stderr) != "problem" {
		t.Errorf("Stderr = %q, want %q", cmdErr.Stderr, "problem")
	}
	if strings.TrimSpace(cmdErr.Stdout) != "out" {
		t.Errorf("Stdout = %q, want %q", cmdErr.Stdout, "out")
	}

	// The exit status must stay reachable through the wrapping.
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatal("exec.ExitError not reachable via errors.As")
	}
	if exitErr.ExitCode() != 3 {
		t.Errorf("exit code = %d, want 3", exitErr.ExitCode())
	}

	// Output() is what callers match tool diagnostics against.
	if out := Output(err); !strings.Contains(out, "problem") || !strings.Contains(out, "out") {
		t.Errorf("Output() = %q, want both streams", out)
	}
	if msg := err.Error(); !strings.Contains(msg, "sh") || !strings.Contains(msg, "problem") {
		t.Errorf("Error() = %q, want command name and output", msg)
	}
}

func TestRunMissingBinary(t *testing.T) {
	_, err := Run(context.Background(), "zapp-no-such-tool-exists")
	if err == nil {
		t.Fatal("Run() with a missing binary returned no error")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("error = %v, want it to wrap exec.ErrNotFound", err)
	}
}

func TestRunRespectsContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Run(ctx, "sleep", "10")
	if err == nil {
		t.Fatal("Run() returned no error for a cancelled command")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Run() took %v; context did not cancel the command", elapsed)
	}
	// The reason must survive, not just "signal: killed".
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want it to wrap context.DeadlineExceeded", err)
	}
}

func TestOutputIgnoresOtherErrors(t *testing.T) {
	if got := Output(errors.New("plain")); got != "" {
		t.Errorf("Output(non-command error) = %q, want empty", got)
	}
	if got := Output(nil); got != "" {
		t.Errorf("Output(nil) = %q, want empty", got)
	}
}
