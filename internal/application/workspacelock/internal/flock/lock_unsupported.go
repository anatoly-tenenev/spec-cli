//go:build !unix

// Package flock isolates the platform-specific advisory file lock. The unix
// build uses a non-blocking flock so a busy workspace reports a concurrency
// conflict immediately; platforms without it report the lock as an unsupported
// capability rather than silently running unlocked.
package flock

import domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"

type Lock struct{}

func AcquireExclusive(lockPath string) (*Lock, *domainerrors.AppError) {
	_ = lockPath
	return nil, domainerrors.New(
		domainerrors.CodeCapabilityUnsupported,
		"workspace lock is not supported on this platform",
		nil,
	)
}

func (l *Lock) Release() {
	_ = l
}
