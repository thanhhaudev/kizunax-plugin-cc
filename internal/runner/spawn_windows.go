//go:build windows

package runner

import "syscall"

// detachSysProcAttr is a no-op on Windows. CreateProcess already
// detaches the child by default (no equivalent of Unix process groups),
// so we return nil. Symmetry with spawn_unix.go keeps the call site in
// runner.go platform-agnostic.
func detachSysProcAttr() *syscall.SysProcAttr {
	return nil
}
