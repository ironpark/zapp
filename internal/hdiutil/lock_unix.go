//go:build unix

package hdiutil

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// lockName is fixed rather than derived from the build, so that every test
// binary of every package contends for the same lock.
const lockName = "zapp-hdiutil.lock"

// lock blocks until no other process holds the lock and returns the function
// that releases it. Closing the file releases the lock as well, so a test
// binary killed mid-command cannot wedge the ones waiting behind it.
func lock() (func(), error) {
	f, err := os.OpenFile(filepath.Join(os.TempDir(), lockName), os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		// The runtime preempts a thread parked in flock with a signal, which
		// surfaces as EINTR rather than as the lock being unavailable.
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() { _ = f.Close() }, nil
}
