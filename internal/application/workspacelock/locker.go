// Package workspacelock serializes mutating commands against one workspace by
// taking an exclusive lock on .spec-cli/workspace.lock. Every write command
// takes it before reading the workspace, so a concurrent run fails loudly with
// a conflict instead of interleaving reads and writes.
package workspacelock

import (
	"path/filepath"
	"sync"

	"github.com/anatoly-tenenev/spec-cli/internal/application/workspacelock/internal/flock"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const (
	lockDirName  = ".spec-cli"
	lockFileName = "workspace.lock"
)

type Guard struct {
	lock        *flock.Lock
	releaseOnce sync.Once
}

func AcquireExclusive(workspacePath string) (*Guard, *domainerrors.AppError) {
	lockPath := filepath.Join(workspacePath, lockDirName, lockFileName)
	acquiredLock, acquireErr := flock.AcquireExclusive(lockPath)
	if acquireErr != nil {
		return nil, acquireErr
	}
	return &Guard{lock: acquiredLock}, nil
}

func (g *Guard) Release() {
	if g == nil {
		return
	}

	g.releaseOnce.Do(func() {
		if g.lock != nil {
			g.lock.Release()
		}
	})
}
