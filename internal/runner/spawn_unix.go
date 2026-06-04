//go:build !windows

package runner

import "syscall"

// detachSysProcAttr returns the SysProcAttr that puts a child in its own
// process group so the parent's SIGINT/SIGTERM does not cascade. Used by
// spawnBackgroundIndexSync to keep the cold-build subprocess alive after
// the foreground review returns.
//
// We intentionally avoid Setsid here — under the Claude Code sandbox on
// macOS, setsid fails with EPERM. Setpgid alone is sufficient for
// signal-isolation in our use case.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
