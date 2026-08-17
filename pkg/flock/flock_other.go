//go:build !unix

package flock

import (
	"fmt"
	"os"
	"runtime"
)

// The advisory flock syscall is unix-only. Refusing keeps a caller from
// reading a no-op as a held lock, which nil would.

func exclusive(_ *os.File) error {
	return fmt.Errorf("%w (GOOS=%s)", ErrUnsupported, runtime.GOOS)
}

func unlock(_ *os.File) error {
	return fmt.Errorf("%w (GOOS=%s)", ErrUnsupported, runtime.GOOS)
}
