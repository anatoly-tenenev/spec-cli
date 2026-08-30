// Package iofailure names filesystem failures for the response. `reason` is a
// small, stable vocabulary a caller can branch on; `detail` is the original
// message, which carries the path but differs per platform and must therefore
// never be the field consumers key off.
//
// Reading documents and taking the workspace lock fail for the same reasons -
// the directory is not there, or it is not ours to write - and a caller sees
// both as an error payload from the same command. Naming them apart would make
// the same missing workspace look like two different problems depending on how
// far the command got.
package iofailure

import (
	"errors"
	"os"
)

// Details describes a filesystem error twice: once for a program, once for a
// person.
func Details(err error) map[string]any {
	return map[string]any{
		"reason": Reason(err),
		"detail": err.Error(),
	}
}

// Reason maps a filesystem error onto the stable vocabulary. It compares the
// kernel error number rather than the message, so the result is the same on
// every platform while err.Error() is not.
func Reason(err error) string {
	switch {
	case errors.Is(err, os.ErrPermission):
		return "permission denied"
	case errors.Is(err, os.ErrNotExist):
		return "not found"
	default:
		return "i/o error"
	}
}
