// Package flock wraps the BSD advisory whole-file lock (flock(2)) so a
// consumer can serialise mutually-exclusive work across processes that
// share a lock file - e.g. one warm-cache writer at a time. The lock is
// advisory: it only constrains cooperating processes that also call here.
//
// Exclusive blocks until it holds the lock; Unlock releases it. Both take
// the *os.File the caller opened on the shared lock path and keep open for
// the lock's lifetime.
//
// Locking is unix-only. A non-unix caller is refused with [ErrUnsupported]
// rather than handed a no-op that reports success. See docs/ward-helpers.md.
package flock

import (
	"errors"
	"os"
)

// ErrUnsupported reports that this platform has no advisory file locking, so
// no lock was taken. Distinct from contention: nobody else holds it.
var ErrUnsupported = errors.New("flock: advisory file locking is unix-only")

// Exclusive takes a blocking exclusive (LOCK_EX) advisory lock on f, held
// until Unlock or f is closed. Non-unix returns [ErrUnsupported].
func Exclusive(f *os.File) error { return exclusive(f) }

// Unlock releases the advisory lock held on f (LOCK_UN). Releasing a file
// that holds no lock is harmless. Non-unix returns [ErrUnsupported].
func Unlock(f *os.File) error { return unlock(f) }
