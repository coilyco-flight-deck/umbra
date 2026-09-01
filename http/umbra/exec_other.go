//go:build windows

// Process spawn, since Windows has no syscall.Exec: run spawns the binary as
// a child, mirrors stdio, and propagates its exit code.
package umbra

import (
	"errors"
	"os"
	"os/exec"
)

// execBinary runs path as a child with inherited stdio and exits with its code,
// the closest Windows analog to replacing the process image.
func execBinary(path string, args []string) error {
	cmd := exec.Command(path, args...) //nolint:gosec // path is the driver-built consumer binary
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}
