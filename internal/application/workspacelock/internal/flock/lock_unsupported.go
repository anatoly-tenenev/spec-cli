//go:build !unix

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
