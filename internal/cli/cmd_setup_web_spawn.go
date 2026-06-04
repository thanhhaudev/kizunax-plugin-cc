//go:build !windows

package cli

import (
	"net"
	"os"
	"os/exec"
	"syscall"

	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
)

// spawnSetupWebWorker forks-execs the current binary as
// "internal-setup-web-worker <token>", passing ln as fd 3 to the child via
// ExtraFiles. The child is detached (Setpgid) and inherits no stdio.
// Returns the child PID on success.
func spawnSetupWebWorker(ln net.Listener, token string) (int, error) {
	tcpLn, ok := ln.(*net.TCPListener)
	if !ok {
		return 0, xerrors.Internal("listener_cast", "listener is not a *net.TCPListener", nil)
	}
	f, err := tcpLn.File()
	if err != nil {
		return 0, xerrors.Internal("listener_file", "cannot extract listener fd", err)
	}
	// f is a *dup* of the underlying fd. Closing it in the parent is required so
	// the child holds the only reference.
	defer f.Close()

	self, err := os.Executable()
	if err != nil {
		return 0, xerrors.Internal("self_exe", "cannot resolve binary path", err)
	}

	cmd := exec.Command(self, "internal-setup-web-worker", token)
	cmd.ExtraFiles = []*os.File{f} // child sees this as fd 3
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // matches internal/job/spawn.go pattern, CC-sandbox-safe
	}

	if err := cmd.Start(); err != nil {
		return 0, xerrors.Internal("spawn", "cannot start worker process", err)
	}
	return cmd.Process.Pid, nil
}
