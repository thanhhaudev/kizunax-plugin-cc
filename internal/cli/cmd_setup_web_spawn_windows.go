//go:build windows

package cli

import (
	"net"
	"os"
	"os/exec"
	"syscall"

	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
)

func spawnSetupWebWorker(ln net.Listener, token string) (int, error) {
	tcpLn, ok := ln.(*net.TCPListener)
	if !ok {
		return 0, xerrors.Internal("listener_cast", "listener is not a *net.TCPListener", nil)
	}
	f, err := tcpLn.File()
	if err != nil {
		return 0, xerrors.Internal("listener_file", "cannot extract listener fd", err)
	}
	defer f.Close()

	self, err := os.Executable()
	if err != nil {
		return 0, xerrors.Internal("self_exe", "cannot resolve binary path", err)
	}

	cmd := exec.Command(self, "internal-setup-web-worker", token)
	cmd.ExtraFiles = []*os.File{f}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}

	if err := cmd.Start(); err != nil {
		return 0, xerrors.Internal("spawn", "cannot start worker process", err)
	}
	return cmd.Process.Pid, nil
}
