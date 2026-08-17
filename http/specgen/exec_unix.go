//go:build !windows

// Process replacement (syscall.Exec) on Unix. Cache locking lives in
// pkg/flock, which is honest about the platforms it does not cover.
package specgen

import (
	"os"
	"syscall"
)

// execBinary replaces the current process with path, passing args through.
// It returns only on failure to exec; on success the process image is gone.
func execBinary(path string, args []string) error {
	return syscall.Exec(path, append([]string{path}, args...), os.Environ())
}
