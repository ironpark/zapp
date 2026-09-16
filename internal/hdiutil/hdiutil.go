// Package hdiutil runs macOS disk image commands one at a time, machine wide.
//
// Two test binaries that use hdiutil at the same time interfere with each
// other. Attaching an image while another attach is probing fails with "no
// mountable file systems", and a volume whose name is already taken by another
// mounted image makes the path a later detach resolves ambiguous, which surfaces
// as a detach of something that is no longer there. `go test ./...` builds one
// binary per package and runs several of them at once, so the exclusion has to
// outlive any single process, and a lock file is what outlives them all.
//
// Tests are the only callers. The claim covers whole mounted sections rather
// than single commands: see Lock.
package hdiutil

import (
	"os/exec"
	"sync"
)

var (
	mu       sync.Mutex
	depth    int
	unlockFn func()
)

// Lock blocks until no other process is using hdiutil and returns the function
// that gives up the claim. Claims nest, so a helper that runs hdiutil inside a
// section that already holds one does not wait on itself; the file lock is
// taken by the outermost claim and released by it.
//
// Hold a claim for as long as an image stays attached:
//
//	release := hdiutil.Lock()
//	defer release()
//
// The lock is what keeps a test suite's disk images from tripping over one
// another, not something the code under test depends on, so a lock file that
// cannot be opened is reported by proceeding without it rather than by failing.
func Lock() func() {
	mu.Lock()
	defer mu.Unlock()
	if depth == 0 {
		release, err := lock()
		if err != nil {
			release = func() {}
		}
		unlockFn = release
	}
	depth++
	var once sync.Once
	return func() { once.Do(release) }
}

func release() {
	mu.Lock()
	defer mu.Unlock()
	depth--
	if depth == 0 {
		unlockFn()
		unlockFn = nil
	}
}

// Run reports stdout and stderr together, for callers that show the output only
// when the command fails.
func Run(args ...string) ([]byte, error) {
	return run(args, (*exec.Cmd).CombinedOutput)
}

// Output reports stdout alone, for callers that parse what hdiutil printed.
func Output(args ...string) ([]byte, error) {
	return run(args, (*exec.Cmd).Output)
}

func run(args []string, collect func(*exec.Cmd) ([]byte, error)) ([]byte, error) {
	defer Lock()()
	return collect(exec.Command("hdiutil", args...))
}
